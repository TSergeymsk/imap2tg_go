package imap

import (
    "crypto/tls"
    "fmt"
    "log/slog"
    "strings"

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

func (c *Client) EnsureConnected() error {
    if c.conn != nil {
        if err := c.conn.Noop(); err == nil {
            return nil
        }
        c.conn = nil
    }

    addr := c.server
    if !strings.Contains(addr, ":") {
        addr = addr + ":993"
    }

    cl, err := client.DialTLS(addr, &tls.Config{InsecureSkipVerify: true})
    if err != nil {
        return fmt.Errorf("dial: %w", err)
    }

    if err := cl.Login(c.username, c.password); err != nil {
        return fmt.Errorf("login: %w", err)
    }

    c.conn = cl
    c.logger.Info("IMAP connected", "server", addr)
    return nil
}

func (c *Client) Select(mailbox string) (*imap.MailboxStatus, error) {
    if c.conn == nil {
        return nil, fmt.Errorf("not connected")
    }
    return c.conn.Select(mailbox, false)
}

// FetchUIDs возвращает все UID >= start (включая start) с помощью UidSearch
func (c *Client) FetchUIDs(start uint32) ([]uint32, error) {
    if c.conn == nil {
        return nil, fmt.Errorf("not connected")
    }
    seqset := new(imap.SeqSet)
    seqset.AddRange(start, 0) // от start до максимума
    criteria := imap.NewSearchCriteria()
    criteria.UID = seqset
    uids, err := c.conn.UidSearch(criteria)
    if err != nil {
        return nil, err
    }
    return uids, nil
}

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

func (c *Client) Close() error {
    if c.conn != nil {
        return c.conn.Logout()
    }
    return nil
}