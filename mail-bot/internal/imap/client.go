package mailparser

import (
    "bytes"
    "encoding/base64"
    "fmt"
    "io"
    "mime"
    "mime/multipart"
    "mime/quotedprintable"
    "net/mail"
    "net/textproto"
    "strings"

    "github.com/PuerkitoBio/goquery"
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
    Text        string
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

// htmlToPlain конвертирует HTML в plain text, удаляя скрипты и стили
func htmlToPlain(html string) string {
    doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
    if err != nil {
        return html
    }
    doc.Find("script, style").Remove()
    text := doc.Text()
    text = strings.ReplaceAll(text, "\u00a0", " ")
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

// getCharset извлекает кодировку из Content-Type
func getCharset(contentType string) string {
    _, params, err := mime.ParseMediaType(contentType)
    if err != nil {
        return "utf-8"
    }
    charset := strings.ToLower(params["charset"])
    if charset == "" {
        charset = "utf-8"
    }
    return charset
}

// decodeCharset преобразует данные из указанной кодировки в UTF-8
func decodeCharset(data []byte, charset string) (string, error) {
    if charset == "utf-8" || charset == "utf8" {
        return string(data), nil
    }
    var decoder *charmap.Charmap
    switch charset {
    case "windows-1251", "cp1251":
        decoder = charmap.Windows1251
    case "koi8-r":
        decoder = charmap.KOI8R
    default:
        return string(data), nil
    }
    if decoder == nil {
        return string(data), nil
    }
    reader := transform.NewReader(bytes.NewReader(data), decoder.NewDecoder())
    decoded, err := io.ReadAll(reader)
    if err != nil {
        return string(data), nil
    }
    return string(decoded), nil
}

// parseContent читает тело части и декодирует с учётом Content-Transfer-Encoding и charset
func parseContent(body io.Reader, header textproto.MIMEHeader) (string, error) {
    ctype := header.Get("Content-Type")
    charset := getCharset(ctype)
    encoding := header.Get("Content-Transfer-Encoding")
    var reader io.Reader = body

    switch encoding {
    case "base64":
        reader = base64.NewDecoder(base64.StdEncoding, reader)
    case "quoted-printable":
        reader = quotedprintable.NewReader(reader)
    }

    data, err := io.ReadAll(reader)
    if err != nil {
        return "", err
    }
    return decodeCharset(data, charset)
}

// extractFromPart рекурсивно обходит части и извлекает текст и вложения
func extractFromPart(part *multipart.Part, email *Email) error {
    // Если часть имеет тип multipart, обрабатываем вложенные части
    ctype := part.Header.Get("Content-Type")
    mediaType, params, err := mime.ParseMediaType(ctype)
    if err == nil && strings.HasPrefix(mediaType, "multipart/") {
        boundary, ok := params["boundary"]
        if !ok {
            return fmt.Errorf("no boundary in nested multipart")
        }
        mr := multipart.NewReader(part, boundary)
        for {
            p, err := mr.NextPart()
            if err == io.EOF {
                break
            }
            if err != nil {
                return err
            }
            if err := extractFromPart(p, email); err != nil {
                return err
            }
        }
        return nil
    }

    // Обработка обычной части
    disposition := part.Header.Get("Content-Disposition")
    if strings.HasPrefix(disposition, "attachment") {
        filename := part.FileName()
        if filename != "" && !strings.EqualFold(filename, "noname") && !strings.EqualFold(filename, "unnamed") {
            filename = DecodeHeader(filename)
            data, err := io.ReadAll(part)
            if err == nil {
                email.Attachments = append(email.Attachments, Attachment{
                    Filename: filename,
                    Data:     data,
                })
            }
        }
        return nil
    }

    // Текстовая часть
    if strings.HasPrefix(mediaType, "text/") {
        bodyText, err := parseContent(part, part.Header)
        if err == nil {
            if mediaType == "text/plain" && email.Text == "" {
                email.Text = strings.TrimSpace(bodyText)
            } else if mediaType == "text/html" && email.Text == "" {
                email.Text = htmlToPlain(bodyText)
            }
        }
    }
    return nil
}

// Parse разбирает сырое письмо (RFC822) с использованием стандартной библиотеки
func Parse(raw []byte) (*Email, error) {
    r := bytes.NewReader(raw)
    msg, err := mail.ReadMessage(r)
    if err != nil {
        return nil, err
    }

    header := msg.Header
    subject := DecodeHeader(header.Get("Subject"))
    from := header.Get("From")
    fromAddr := from
    fromName := from

    addrs, err := header.AddressList("From")
    if err == nil && len(addrs) > 0 {
        fromName = addrs[0].Name
        fromAddr = addrs[0].Address
        if fromName == "" {
            fromName = fromAddr
        }
        fromName = DecodeHeader(fromName)
    }

    email := &Email{
        Subject:  subject,
        From:     fromAddr,
        FromName: fromName,
    }

    mediaType, params, err := mime.ParseMediaType(header.Get("Content-Type"))
    if err == nil && strings.HasPrefix(mediaType, "multipart/") {
        boundary, ok := params["boundary"]
        if !ok {
            return nil, fmt.Errorf("no boundary in multipart")
        }
        mr := multipart.NewReader(msg.Body, boundary)
        for {
            part, err := mr.NextPart()
            if err == io.EOF {
                break
            }
            if err != nil {
                return nil, err
            }
            if err := extractFromPart(part, email); err != nil {
                // продолжаем, но можно залогировать
                continue
            }
        }
    } else {
        // Не multipart
        bodyText, err := parseContent(msg.Body, textproto.MIMEHeader(header))
        if err == nil {
            if strings.HasPrefix(mediaType, "text/plain") || mediaType == "" {
                email.Text = strings.TrimSpace(bodyText)
            } else if strings.HasPrefix(mediaType, "text/html") {
                email.Text = htmlToPlain(bodyText)
            } else {
                email.Text = "Нет текста письма"
            }
        } else {
            email.Text = "Нет текста письма"
        }
    }

    return email, nil
}