package imap

import (
    "crypto/tls"
    "fmt"
    "log/slog"
    "time"

    "github.com/emersion/go-imap"
    "github.com/emersion/go-imap/client"
)

type Client struct {
    server   string
    username string
    password string
    conn     *client.Client
    logger   *slog.Logger
}

func New(server, username, password string, logger *slog.Logger) *Client {
    return &Client{
        server:   server,
        username: username,
        password: password,
        logger:   logger,
    }
}

// EnsureConnected устанавливает соединение, если его нет или оно разорвано
func (c *Client) EnsureConnected() error {
    if c.conn != nil {
        // Проверяем живучесть через Noop
        if err := c.conn.Noop(); err == nil {
            return nil
        }
        c.conn = nil
    }

    // Подключаемся с TLS
    cl, err := client.DialTLS(c.server, &tls.Config{InsecureSkipVerify: true})
    if err != nil {
        return fmt.Errorf("dial: %w", err)
    }

    if err := cl.Login(c.username, c.password); err != nil {
        return fmt.Errorf("login: %w", err)
    }

    c.conn = cl
    c.logger.Info("IMAP connected", "server", c.server)
    return nil
}

// Select выбирает папку (INBOX)
func (c *Client) Select(mailbox string) (*imap.MailboxStatus, error) {
    if c.conn == nil {
        return nil, fmt.Errorf("not connected")
    }
    status, err := c.conn.Select(mailbox, false)
    if err != nil {
        return nil, err
    }
    return status, nil
}

// FetchUIDs возвращает все UID в диапазоне от start до 0 (бесконечность)
func (c *Client) FetchUIDs(start uint32) ([]uint32, error) {
    if c.conn == nil {
        return nil, fmt.Errorf("not connected")
    }
    seqset := new(imap.SeqSet)
    seqset.AddRange(start, 0) // 0 означает максимум

    // Ищем UID
    messages := make(chan *imap.Message, 100)
    done := make(chan error, 1)
    go func() {
        done <- c.conn.UidFetch(seqset, []imap.FetchItem{imap.FetchItem("UID")}, messages)
    }()

    var uids []uint32
    for msg := range messages {
        uids = append(uids, msg.Uid)
    }
    if err := <-done; err != nil {
        return nil, err
    }
    return uids, nil
}

// FetchMessage загружает полное письмо по UID
func (c *Client) FetchMessage(uid uint32) (*imap.Message, error) {
    if c.conn == nil {
        return nil, fmt.Errorf("not connected")
    }
    seqset := new(imap.SeqSet)
    seqset.AddNum(uid)

    messages := make(chan *imap.Message, 1)
    done := make(chan error, 1)
    go func() {
        done <- c.conn.UidFetch(seqset, []imap.FetchItem{imap.FetchItem("RFC822")}, messages)
    }()

    msg := <-messages
    if err := <-done; err != nil {
        return nil, err
    }
    return msg, nil
}

// Close закрывает соединение
func (c *Client) Close() error {
    if c.conn != nil {
        return c.conn.Logout()
    }
    return nil
}