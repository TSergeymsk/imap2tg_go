package mailparser

import (
    "strings"
    "testing"
)

func TestParseSimplePlainText(t *testing.T) {
    raw := []byte("From: sender@example.com\r\n" +
        "Subject: Test Subject\r\n" +
        "Content-Type: text/plain; charset=utf-8\r\n" +
        "\r\n" +
        "Hello, this is a test message.")

    email, err := Parse(raw)
    if err != nil {
        t.Fatalf("Parse failed: %v", err)
    }

    if email.Subject != "Test Subject" {
        t.Errorf("Subject = %s, want 'Test Subject'", email.Subject)
    }
    if email.From != "sender@example.com" {
        t.Errorf("From = %s, want sender@example.com", email.From)
    }
    if email.Text != "Hello, this is a test message." {
        t.Errorf("Text = %s, want 'Hello, this is a test message.'", email.Text)
    }
    if len(email.Attachments) != 0 {
        t.Errorf("Attachments count = %d, want 0", len(email.Attachments))
    }
}

func TestParseMultipartAlternative(t *testing.T) {
    raw := []byte("From: sender@example.com\r\n" +
        "Subject: Multipart Test\r\n" +
        "Content-Type: multipart/alternative; boundary=boundary123\r\n" +
        "\r\n" +
        "--boundary123\r\n" +
        "Content-Type: text/plain; charset=utf-8\r\n" +
        "\r\n" +
        "Plain text version.\r\n" +
        "--boundary123\r\n" +
        "Content-Type: text/html; charset=utf-8\r\n" +
        "\r\n" +
        "<html><body><p>HTML version</p></body></html>\r\n" +
        "--boundary123--\r\n")

    email, err := Parse(raw)
    if err != nil {
        t.Fatalf("Parse failed: %v", err)
    }

    if email.Subject != "Multipart Test" {
        t.Errorf("Subject = %s, want 'Multipart Test'", email.Subject)
    }
    if email.Text != "Plain text version." {
        t.Errorf("Text = %s, want 'Plain text version.'", email.Text)
    }
    if len(email.Attachments) != 0 {
        t.Errorf("Attachments count = %d, want 0", len(email.Attachments))
    }
}

func TestParseWithAttachment(t *testing.T) {
    raw := []byte("From: sender@example.com\r\n" +
        "Subject: With Attachment\r\n" +
        "Content-Type: multipart/mixed; boundary=boundary123\r\n" +
        "\r\n" +
        "--boundary123\r\n" +
        "Content-Type: text/plain; charset=utf-8\r\n" +
        "\r\n" +
        "This is the body.\r\n" +
        "--boundary123\r\n" +
        "Content-Type: application/octet-stream\r\n" +
        "Content-Disposition: attachment; filename=\"test.txt\"\r\n" +
        "\r\n" +
        "file content\r\n" +
        "--boundary123--\r\n")

    email, err := Parse(raw)
    if err != nil {
        t.Fatalf("Parse failed: %v", err)
    }

    if email.Text != "This is the body." {
        t.Errorf("Text = %s, want 'This is the body.'", email.Text)
    }
    if len(email.Attachments) != 1 {
        t.Fatalf("Attachments count = %d, want 1", len(email.Attachments))
    }
    att := email.Attachments[0]
    if att.Filename != "test.txt" {
        t.Errorf("Attachment filename = %s, want test.txt", att.Filename)
    }
    if string(att.Data) != "file content" {
        t.Errorf("Attachment data = %s, want 'file content'", string(att.Data))
    }
}

func TestParseNestedMultipart(t *testing.T) {
    raw := []byte("From: sender@example.com\r\n" +
        "Subject: Nested\r\n" +
        "Content-Type: multipart/mixed; boundary=outer\r\n" +
        "\r\n" +
        "--outer\r\n" +
        "Content-Type: multipart/alternative; boundary=inner\r\n" +
        "\r\n" +
        "--inner\r\n" +
        "Content-Type: text/plain; charset=utf-8\r\n" +
        "\r\n" +
        "Nested plain text\r\n" +
        "--inner--\r\n" +
        "--outer--\r\n")

    email, err := Parse(raw)
    if err != nil {
        t.Fatalf("Parse failed: %v", err)
    }

    if email.Text != "Nested plain text" {
        t.Errorf("Text = %s, want 'Nested plain text'", email.Text)
    }
}

func TestDecodeHeader(t *testing.T) {
    encoded := "=?UTF-8?B?0JHRg9C60LLQsA==?="
    decoded := DecodeHeader(encoded)
    if decoded != "Буква" {
        t.Errorf("DecodeHeader = %s, want 'Буква'", decoded)
    }
}

func TestParseHTMLToPlain(t *testing.T) {
    html := "<html><head><style>body{color:red;}</style></head><body><p>Hello <b>world</b></p><script>alert(1)</script></body></html>"
    plain := htmlToPlain(html)
    expected := "Hello world"
    if plain != expected {
        t.Errorf("htmlToPlain = %s, want '%s'", plain, expected)
    }
}