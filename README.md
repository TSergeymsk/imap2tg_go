# Mail Bot для Telegram

Бот для автоматической пересылки новых входящих писем из IMAP-почтового ящика в чат Telegram. Написан на Go — легковесный, быстрый, с низким потреблением памяти.

## Возможности

- Мониторинг папки **INBOX** через IMAP over SSL.
- Отправка в Telegram:
  - Тема письма, отправитель, текст (plain text или из HTML).
  - Список вложений с именами.
  - Сами вложения как файлы (опционально).
- Хранение последнего обработанного UID **в описании чата Telegram** — состояние сохраняется между перезапусками.
- Автоматическое переподключение при обрыве IMAP-соединения.
- Работа через HTTP-прокси (например, Privoxy).
- Гибкое логирование (уровни, файл + консоль).
- Простая конфигурация в JSON.

## Требования

- Go 1.21 или новее (для сборки).
- Доступ к IMAP-серверу по TLS (порт 993).
- Токен бота Telegram и идентификатор чата.

## Установка

### Из исходников

```bash
git clone https://github.com/yourusername/mail-bot.git
cd mail-bot
go mod download
go build -ldflags="-s -w" -o mail-bot cmd/mail-bot/main.go
```

Получится исполняемый файл `mail-bot` (или `mail-bot.exe`).

### Готовый бинарник

Если не хотите собирать сами, скачайте релиз для вашей платформы с [страницы релизов](https://github.com/yourusername/mail-bot/releases).

## Конфигурация

Создайте файл `config.json` (можно скопировать из `config.json.example`) со следующими полями:

| Поле | Описание |
|------|----------|
| `imap_server` | Адрес IMAP-сервера с портом (например, `imap.example.com:993`) |
| `username` | Логин для почты |
| `password` | Пароль от почты |
| `bot_token` | Токен Telegram-бота (получить у [@BotFather](https://t.me/botfather)) |
| `chat_id` | ID чата/пользователя, куда отправлять письма |
| `chat_id_err` | (опционально) ID чата для отправки критических ошибок |
| `encoding` | Кодировка для IMAP-команд, обычно `UTF-8` |
| `send_attach` | `true` / `false` – отправлять ли файлы-вложения |
| `proxy_url` | (опционально) HTTP-прокси, например `http://proxy:8118` |
| `logfile` | Путь к файлу логов |
| `loglevel` | Уровень логирования: `debug`, `info`, `warn`, `error` |

**Пример** (`config.json`):

```json
{
    "imap_server": "imap.gmail.com:993",
    "username": "my@gmail.com",
    "password": "app_password",
    "bot_token": "123456:ABC-DEF1234ghIkl",
    "chat_id": 123456789,
    "send_attach": true,
    "logfile": "mail-bot.log",
    "loglevel": "info"
}
```

### Миграция со старого Python-конфига

Если у вас есть файл `config.py` от предыдущей версии (Python), используйте конвертер:

```bash
python3 convert_config.py /путь/к/config.py
```

Скрипт создаст `config.json` в текущей папке.

## Запуск

```bash
./mail-bot config.json
```

Для работы в фоне рекомендуется использовать systemd (пример ниже) или screen/tmux.

## Systemd-сервис

Создайте файл `/etc/systemd/system/mail-bot.service`:

```ini
[Unit]
Description=Mail Bot for Telegram
After=network.target

[Service]
Type=simple
User=root
WorkingDirectory=/opt/mail-bot
ExecStart=/opt/mail-bot/mail-bot /opt/mail-bot/config.json
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Затем:

```bash
sudo systemctl daemon-reload
sudo systemctl enable mail-bot
sudo systemctl start mail-bot
```

## Как это работает

1. Бот запускается и читает последний обработанный UID из описания чата Telegram.
2. Подключается к IMAP-серверу, выбирает папку INBOX.
3. Каждые 15 секунд опрашивает почту на наличие новых писем с UID > last_id.
4. Для каждого нового письма:
   - Загружает его.
   - Парсит заголовки, текст (HTML → plain), вложения.
   - Отправляет основное сообщение в Telegram.
   - Если `send_attach: true` – отправляет все вложения как документы.
   - Обновляет last_id и сохраняет его в описании чата.
5. При сбое IMAP переподключается автоматически.
6. Остановка по Ctrl+C или SIGTERM – корректное закрытие соединений.

## Архитектура проекта

```
mail-bot/
├── cmd/mail-bot/main.go       # точка входа, основной цикл
├── internal/
│   ├── config/                # загрузка JSON
│   ├── imap/                  # клиент IMAP с переподключением
│   ├── mailparser/            # разбор письма, вложений, HTML→text
│   ├── telegram/              # клиент Telegram API
│   └── logger/                # настройка slog
├── go.mod
├── config.json.example
└── convert_config.py          # конвертер старого config.py
```

## Отладка

- Логи пишутся в файл, указанный в `logfile`, и одновременно в stdout.
- Уровень `debug` покажет детальную информацию о каждом шаге.
- Если что-то пошло не так, проверьте логи и доступность IMAP/Telegram.

## Известные ограничения

- Поддерживаются только IMAP-серверы с TLS (без STARTTLS).
- Размер вложений ограничен 50 МБ (лимит Telegram).
- HTML-конвертация удаляет теги `<script>` и `<style>`, остальное преобразует в текст.

## Вклад в проект

Приветствуются pull requests и issues. При добавлении новых кодировок или провайдеров – обновляйте документацию.

## Лицензия

MIT License. Свободно используйте, модифицируйте и распространяйте.

---

**Удачи в использовании!** Если появятся вопросы – создавайте issue.