package mailparser

import (
    "bytes"
    "io"
    "mime"
    "strings"

    "github.com/PuerkitoBio/goquery"
    "github.com/emersion/go-message/mail"
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

func DecodeHeader(h string) string {
    decoder := new(mime.WordDecoder)
    decoded, err := decoder.DecodeHeader(h)
    if err != nil {
        return h
    }
    return decoded
}

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

func decodeCharset(data []byte, charset string) (string, error) {
    if charset == "utf-8" || charset == "utf8" {
        return string(data), nil
    }
    var decoder transform.Transformer
    switch charset {
    case "windows-1251", "cp1251":
        decoder = charmap.Windows1251.NewDecoder()
    case "koi8-r":
        decoder = charmap.KOI8R.NewDecoder()
    default:
        if enc, ok := charmap.EncodingForName(charset); ok {
            decoder = enc.NewDecoder()
        } else {
            return string(data), nil
        }
    }
    reader := transform.NewReader(bytes.NewReader(data), decoder)
    decoded, err := io.ReadAll(reader)
    if err != nil {
        return string(data), nil
    }
    return string(decoded), nil
}

func Parse(raw []byte) (*Email, error) {
    r := bytes.NewReader(raw)
    mr, err := mail.NewReader(r)
    if err != nil {
        return nil, err
    }

    header := mr.Header
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

    for {
        p, err := mr.NextPart()
        if err == io.EOF {
            break
        }
        if err != nil {
            return nil, err
        }

        switch h := p.Header.(type) {
        case *mail.InlineHeader:
            contentType := h.Get("Content-Type")
            mediaType, _, _ := mime.ParseMediaType(contentType)
            body, err := io.ReadAll(p.Body)
            if err != nil {
                continue
            }
            charset := getCharset(contentType)
            decoded, _ := decodeCharset(body, charset)

            if mediaType == "text/plain" {
                email.Text = strings.TrimSpace(decoded)
            } else if mediaType == "text/html" && email.Text == "" {
                email.Text = htmlToPlain(decoded)
            }

        case *mail.AttachmentHeader:
            filename := h.Filename()
            if filename != "" &&
                !strings.EqualFold(filename, "noname") &&
                !strings.EqualFold(filename, "unnamed") {
                filename = DecodeHeader(filename)
                data, err := io.ReadAll(p.Body)
                if err == nil {
                    email.Attachments = append(email.Attachments, Attachment{
                        Filename: filename,
                        Data:     data,
                    })
                }
            }
        }
    }

    return email, nil
}