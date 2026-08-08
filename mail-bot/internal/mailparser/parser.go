package mailparser

import (
    "bytes"
    "encoding/base64"
    "fmt"
    "io"
    "mime"
    "mime/quotedprintable"
    "net/textproto"
    "strings"

    "github.com/PuerkitoBio/goquery"
    "github.com/emersion/go-message"
    "golang.org/x/text/encoding/charmap"
    "golang.org/x/text/transform"
)

type Attachment struct {
    Filename string
    Data     []byte
}

type Email struct {
    Subject     string
    From        string
    FromName    string
    Text        string   // plain text body
    Attachments []Attachment
}

// DecodeHeader декодирует заголовок в соответствии с RFC 2047
func DecodeHeader(h string) string {
    decoder := new(mime.WordDecoder)
    decoded, err := decoder.DecodeHeader(h)
    if err != nil {
        return h
    }
    return decoded
}

// decodePart декодирует тело части с учётом Content-Transfer-Encoding и charset
func decodePart(body io.Reader, header textproto.MIMEHeader) (string, error) {
    encoding := header.Get("Content-Transfer-Encoding")
    var reader io.Reader = body

    switch encoding {
    case "base64":
        reader = base64.NewDecoder(base64.StdEncoding, reader)
    case "quoted-printable":
        reader = quotedprintable.NewReader(reader)
    }

    // Определяем кодировку символов
    contentType := header.Get("Content-Type")
    _, params, err := mime.ParseMediaType(contentType)
    if err != nil {
        // Если не удалось, используем UTF-8
        params = map[string]string{"charset": "utf-8"}
    }
    charset := strings.ToLower(params["charset"])
    if charset == "" {
        charset = "utf-8"
    }

    // Читаем всё в []byte
    raw, err := io.ReadAll(reader)
    if err != nil {
        return "", err
    }

    // Преобразуем в UTF-8, если нужно
    if charset != "utf-8" && charset != "utf8" {
        var decoder transform.Transformer
        switch charset {
        case "windows-1251", "cp1251":
            decoder = charmap.Windows1251.NewDecoder()
        case "koi8-r":
            decoder = charmap.KOI8R.NewDecoder()
        // Добавьте другие кодировки при необходимости
        default:
            // пробуем стандартный преобразователь
            if enc, ok := charmap.Encoding(charset); ok {
                decoder = enc.NewDecoder()
            } else {
                // оставляем как есть
                return string(raw), nil
            }
        }
        if decoder != nil {
            reader := transform.NewReader(bytes.NewReader(raw), decoder)
            decoded, err := io.ReadAll(reader)
            if err == nil {
                return string(decoded), nil
            }
        }
    }
    return string(raw), nil
}

// htmlToPlain конвертирует HTML в plain text, удаляя скрипты и стили
func htmlToPlain(html string) string {
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
    if err != nil {
        return html
    }
    // Удаляем script и style
    doc.Find("script, style").Remove()
    text := doc.Text()
    // Заменяем неразрывные пробелы
    text = strings.ReplaceAll(text, "\u00a0", " ")
    // Убираем лишние пробелы и пустые строки
    lines := strings.Split(text, "\n")
    var cleanLines []string
    for _, line := range lines {
        trimmed := strings.TrimSpace(line)
        if trimmed != "" {
            cleanLines = append(cleanLines, trimmed)
        }
    }
    return strings.Join(cleanLines, "\n")
}

// Parse разбирает сырое письмо (RFC822)
func Parse(raw []byte) (*Email, error) {
    r := bytes.NewReader(raw)
    mr, err := message.NewReader(r)
    if err != nil {
        return nil, err
    }

    // Заголовки
    h := mr.Header
    subject := DecodeHeader(h.Get("Subject"))
    from := h.Get("From")
    fromAddr := from
    fromName := from
    // Попробуем извлечь имя и email
    if addrs, err := h.AddressList("From"); err == nil && len(addrs) > 0 {
        fromName = addrs[0].Name
        fromAddr = addrs[0].Address
        if fromName == "" {
            fromName = fromAddr
        }
    }
    fromName = DecodeHeader(fromName)

    email := &Email{
        Subject:  subject,
        From:     fromAddr,
        FromName: fromName,
    }

    // Обходим все части
    for {
        p, err := mr.NextPart()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }

        switch part := p.(type) {
        case *message.Part:
            // Вложение
            filename := part.FileName()
            if filename != "" {
                // Пропускаем служебные имена (например, "noname")
                if strings.EqualFold(filename, "noname") || strings.EqualFold(filename, "unnamed") {
                    continue
                }
                // Декодируем имя (может быть закодировано)
                filename = DecodeHeader(filename)
                data, err := io.ReadAll(part.Body)
                if err != nil {
                    continue
                }
                email.Attachments = append(email.Attachments, Attachment{
                    Filename: filename,
                    Data:     data,
                })
            }

        case *message.Entity:
            // Текстовая часть (text/plain или text/html)
            ctype := part.Header.Get("Content-Type")
            mediaType, _, _ := mime.ParseMediaType(ctype)
            if mediaType == "text/plain" {
                body, err := decodePart(part.Body, part.Header)
                if err == nil {
                    email.Text = body
                }
            } else if mediaType == "text/html" {
                body, err := decodePart(part.Body, part.Header)
                if err == nil {
                    // Если ещё нет plain text, используем HTML (преобразованный)
                    if email.Text == "" {
                        email.Text = htmlToPlain(body)
                    }
                }
            }
        }
    }

    // Если текст пустой, пробуем взять весь тело как есть (для однопартсных писем)
    if email.Text == "" {
        // Если письмо не multipart, то всё тело – это текст
        // В таком случае mr.Body содержит его
        body, err := decodePart(mr.Body, mr.Header)
        if err == nil {
            ctype := mr.Header.Get("Content-Type")
            mediaType, _, _ := mime.ParseMediaType(ctype)
            if mediaType == "text/html" {
                email.Text = htmlToPlain(body)
            } else {
                email.Text = body
            }
        }
    }

    return email, nil
}