package mailparser

import (
    "bytes"
    "io"
    "mime"
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
        // Неизвестная кодировка – возвращаем как есть
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

// Parse разбирает сырое письмо (RFC822)
func Parse(raw []byte) (*Email, error) {
    r := bytes.NewReader(raw)
    entity, err := message.NewReader(r)
    if err != nil {
        return nil, err
    }

    header := entity.Header
    subject := DecodeHeader(header.Get("Subject"))
    from := header.Get("From")
    fromAddr := from
    fromName := from

    if addrs, err := header.AddressList("From"); err == nil && len(addrs) > 0 {
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

    // Если письмо multipart, обрабатываем части
    if entity.Multipart() {
        for {
            part, err := entity.NextPart()
            if err == io.EOF {
                break
            }
            if err != nil {
                return nil, err
            }

            ctype := part.Header.Get("Content-Type")
            mediaType, _, _ := mime.ParseMediaType(ctype)

            // Проверяем, является ли часть вложением
            disposition := part.Header.Get("Content-Disposition")
            if strings.HasPrefix(disposition, "attachment") {
                filename := part.FileName()
                if filename != "" && !strings.EqualFold(filename, "noname") && !strings.EqualFold(filename, "unnamed") {
                    filename = DecodeHeader(filename)
                    data, err := io.ReadAll(part.Body)
                    if err == nil {
                        email.Attachments = append(email.Attachments, Attachment{
                            Filename: filename,
                            Data:     data,
                        })
                    }
                }
                continue
            }

            // Обрабатываем текстовые части
            if strings.HasPrefix(mediaType, "text/") {
                body, err := io.ReadAll(part.Body)
                if err != nil {
                    continue
                }
                charset := getCharset(ctype)
                decoded, _ := decodeCharset(body, charset)

                if mediaType == "text/plain" {
                    email.Text = strings.TrimSpace(decoded)
                } else if mediaType == "text/html" && email.Text == "" {
                    email.Text = htmlToPlain(decoded)
                }
            }
        }
    } else {
        // Не multipart – читаем тело
        body, err := io.ReadAll(entity.Body)
        if err != nil {
            return nil, err
        }
        ctype := header.Get("Content-Type")
        mediaType, _, _ := mime.ParseMediaType(ctype)
        charset := getCharset(ctype)
        decoded, _ := decodeCharset(body, charset)
        if mediaType == "text/plain" {
            email.Text = strings.TrimSpace(decoded)
        } else if mediaType == "text/html" {
            email.Text = htmlToPlain(decoded)
        } else {
            email.Text = "Нет текста письма"
        }
    }

    return email, nil
}