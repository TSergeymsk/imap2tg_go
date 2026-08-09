package imap

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/emersion/go-imap"
	"github.com/emersion/go-imap/client"

	"mail-bot/internal/config"
)

type Client struct {
	server    string
	username  string
	password  string
	tlsConfig *tls.Config
	conn      *client.Client
	logger    *slog.Logger
}

// New создаёт IMAP-клиент с TLS-настройками из конфига.
func New(server, username, password string, cfg *config.Config, logger *slog.Logger) (*Client, error) {
	tlsCfg, err := buildTLSConfig(cfg, logger)
	if err != nil {
		return nil, fmt.Errorf("build TLS config: %w", err)
	}

	return &Client{
		server:    server,
		username:  username,
		password:  password,
		tlsConfig: tlsCfg,
		logger:    logger,
	}, nil
}

// buildTLSConfig создаёт tls.Config с системными и дополнительными CA.
func buildTLSConfig(cfg *config.Config, logger *slog.Logger) (*tls.Config, error) {
	// 1. Системный пул CA
	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		rootCAs = x509.NewCertPool()
		logger.Warn("failed to load system CA pool, using empty pool", "err", err)
	}

	// 2. Добавляем CA из файла
	if cfg.TLSCACertFile != "" {
		pem, err := os.ReadFile(cfg.TLSCACertFile)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		if !rootCAs.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("failed to parse CA cert from file: %s", cfg.TLSCACertFile)
		}
		logger.Debug("added CA from file", "path", cfg.TLSCACertFile)
	}

	// 3. Добавляем CA из директории
	if cfg.TLSCACertDir != "" {
		err := filepath.WalkDir(cfg.TLSCACertDir, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			ext := strings.ToLower(filepath.Ext(path))
			if ext != ".pem" && ext != ".crt" && ext != ".cer" {
				return nil
			}
			pem, err := os.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read CA file %s: %w", path, err)
			}
			if !rootCAs.AppendCertsFromPEM(pem) {
				logger.Warn("failed to parse CA cert", "path", path)
			} else {
				logger.Debug("added CA from file", "path", path)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("walk CA dir: %w", err)
		}
	}

	tlsCfg := &tls.Config{
		RootCAs:            rootCAs,
		InsecureSkipVerify: cfg.TLSInsecureSkipVerify,
		ServerName:         strings.Split(cfg.IMAPServer, ":")[0],
	}

	return tlsCfg, nil
}

// EnsureConnected подключается к IMAP-серверу с TLS.
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

	// Используем client.DialTLS, который принимает address и *tls.Config
	cl, err := client.DialTLS(addr, c.tlsConfig)
	if err != nil {
		return fmt.Errorf("dial TLS: %w", err)
	}

	if err := cl.Login(c.username, c.password); err != nil {
		cl.Logout()
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

func (c *Client) FetchUIDs(start uint32) ([]uint32, error) {
	if c.conn == nil {
		return nil, fmt.Errorf("not connected")
	}
	seqset := new(imap.SeqSet)
	if start > 0 {
		seqset.AddRange(start, ^uint32(0))
	} else {
		seqset.AddRange(1, ^uint32(0))
	}
	criteria := imap.NewSearchCriteria()
	criteria.Uid = seqset
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