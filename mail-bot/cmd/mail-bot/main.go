package main

import (
    "context"
    "fmt"
    "log/slog"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    "mail-bot/internal/config"
    "mail-bot/internal/imap"
    "mail-bot/internal/logger"
    "mail-bot/internal/mailparser"
    "mail-bot/internal/telegram"
)

func main() {
    if len(os.Args) < 2 {
        fmt.Println("Usage: mail-bot <config.json>")
        os.Exit(1)
    }
    cfgPath := os.Args[1]

    // Загружаем конфиг
    cfg, err := config.Load(cfgPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
        os.Exit(1)
    }

    // Инициализация логгера
    log := logger.Setup(cfg.LogFile, cfg.LogLevel)

    // Клиент Telegram
    tg := telegram.New(cfg.BotToken, cfg.ProxyURL, log)

    // Контекст для graceful shutdown
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Перехват сигналов
    sigChan := make(chan os.Signal, 1)
    signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigChan
        log.Info("received shutdown signal, stopping...")
        cancel()
    }()

    // Запускаем основной цикл
    if err := run(ctx, cfg, tg, log); err != nil {
        log.Error("fatal error", "err", err)
        os.Exit(1)
    }
    log.Info("mail-bot stopped")
}

func run(ctx context.Context, cfg *config.Config, tg *telegram.Client, log *slog.Logger) error {
    imapClient := imap.New(cfg.IMAPServer, cfg.Username, cfg.Password, log)

    // Получаем последний обработанный UID из описания чата
    lastID := uint32(0)
    for {
        chat, err := tg.GetChat(cfg.ChatID)
        if err != nil {
            log.Error("get chat failed", "err", err)
            time.Sleep(15 * time.Second)
            continue
        }
        if chat.Description != "" {
            if val, err := strconv.ParseUint(chat.Description, 10, 32); err == nil {
                lastID = uint32(val)
                log.Info("loaded last_id from chat description", "last_id", lastID)
                break
            }
        }
        // Если нет описания или оно не число – стартуем с 0
        log.Info("no valid last_id found, starting from 0")
        break
    }

    for {
        select {
        case <-ctx.Done():
            imapClient.Close()
            return nil
        default:
        }

        // Обеспечиваем соединение
        if err := imapClient.EnsureConnected(); err != nil {
            log.Error("imap connection error", "err", err)
            time.Sleep(30 * time.Second)
            continue
        }

        // Выбираем INBOX
        status, err := imapClient.Select("INBOX")
        if err != nil {
            log.Error("select inbox failed", "err", err)
            time.Sleep(30 * time.Second)
            continue
        }
        log.Debug("inbox selected", "exists", status.Messages, "uidnext", status.UidNext)

        // Ищем UID начиная с lastID+1
        uids, err := imapClient.FetchUIDs(lastID + 1)
        if err != nil {
            log.Error("fetch uids failed", "err", err)
            time.Sleep(30 * time.Second)
            continue
        }

        if len(uids) > 0 {
            log.Info("new messages found", "count", len(uids))
            for _, uid := range uids {
                // Обрабатываем письмо
                if err := processMessage(imapClient, tg, cfg, uid, log); err != nil {
                    log.Error("process message failed", "uid", uid, "err", err)
                    continue
                }
                // Обновляем lastID
                if uid > lastID {
                    lastID = uid
                }
                // Сохраняем lastID в описание чата
                if err := tg.SetChatDescription(cfg.ChatID, strconv.FormatUint(uint64(lastID), 10)); err != nil {
                    log.Error("save last_id failed", "uid", lastID, "err", err)
                } else {
                    log.Debug("saved last_id", "uid", lastID)
                }
            }
        }

        // Пауза перед следующим опросом
        time.Sleep(15 * time.Second)
    }
}

// processMessage обрабатывает одно письмо
func processMessage(imapClient *imap.Client, tg *telegram.Client, cfg *config.Config, uid uint32, log *slog.Logger) error {
    // Загружаем письмо
    msg, err := imapClient.FetchMessage(uid)
    if err != nil {
        return fmt.Errorf("fetch message: %w", err)
    }
    raw := msg.RawBody()
    if raw == nil {
        return fmt.Errorf("no raw body")
    }

    // Парсим
    email, err := mailparser.Parse(raw)
    if err != nil {
        return fmt.Errorf("parse: %w", err)
    }

    // Формируем текст сообщения
    text := buildTelegramText(email)

    // Отправляем основное сообщение
    if err := tg.SendMessage(cfg.ChatID, text); err != nil {
        return fmt.Errorf("send message: %w", err)
    }

    // Отправляем вложения, если нужно
    if cfg.SendAttach && len(email.Attachments) > 0 {
        for _, att := range email.Attachments {
            // Ограничиваем размер отправляемого файла (Telegram: до 50 МБ)
            if len(att.Data) > 50*1024*1024 {
                log.Warn("attachment too large, skipping", "file", att.Filename, "size", len(att.Data))
                continue
            }
            if err := tg.SendDocument(cfg.ChatID, att.Filename, att.Data, email.Subject); err != nil {
                log.Error("send attachment failed", "file", att.Filename, "err", err)
            }
        }
    }

    log.Info("message processed", "uid", uid, "subject", email.Subject)
    return nil
}

// buildTelegramText собирает текст для отправки (до 4000 символов)
func buildTelegramText(email *mailparser.Email) string {
    fromDisplay := email.FromName
    if fromDisplay == "" {
        fromDisplay = email.From
    }
    subject := email.Subject
    if subject == "" {
        subject = "Без темы"
    }
    text := email.Text
    if text == "" {
        text = "Нет текста письма"
    }

    // Ограничиваем длину текста, чтобы общая длина не превышала 4000
    const maxLen = 4000
    // Сначала собираем без вложений (вложения только перечисляем)
    attachmentsList := ""
    if len(email.Attachments) > 0 {
        var names []string
        for _, a := range email.Attachments {
            names = append(names, a.Filename)
        }
        attachmentsList = fmt.Sprintf("\n📎 Вложения: %d\n%s", len(email.Attachments), strings.Join(names, "\n"))
    }

    // Собираем шаблон
    full := fmt.Sprintf("📨 <b>%s</b>\n\nОт: <pre>%s (%s)</pre>\n\n%s\n\n%s",
        subject,
        fromDisplay,
        email.From,
        text,
        attachmentsList,
    )

    // Если слишком длинное, обрезаем текст письма
    if len(full) > maxLen {
        // Обрезаем текст, оставляя место для остального
        overhead := len(full) - len(text) // длина шапки + вложений
        if overhead > maxLen-100 {
            // Если шапка слишком большая, просто обрезаем всё
            return full[:maxLen-3] + "..."
        }
        maxTextLen := maxLen - overhead - 3
        if len(text) > maxTextLen {
            text = text[:maxTextLen] + "..."
            // Пересобираем
            full = fmt.Sprintf("📨 <b>%s</b>\n\nОт: <pre>%s (%s)</pre>\n\n%s\n\n%s",
                subject, fromDisplay, email.From, text, attachmentsList)
        }
    }
    return full
}