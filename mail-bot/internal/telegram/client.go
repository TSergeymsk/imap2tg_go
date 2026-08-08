package telegram

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "log/slog"
    "mime/multipart"
    "net/http"
    "net/url"
    "strconv"
    "time"
)

type Client struct {
    botToken string
    httpClient *http.Client
    logger   *slog.Logger
}

func New(botToken, proxyURL string, logger *slog.Logger) *Client {
    transport := &http.Transport{
        MaxIdleConns:    10,
        IdleConnTimeout: 30 * time.Second,
    }
    if proxyURL != "" {
        proxy, err := url.Parse(proxyURL)
        if err == nil {
            transport.Proxy = http.ProxyURL(proxy)
        } else {
            logger.Error("invalid proxy url", "err", err)
        }
    }
    return &Client{
        botToken: botToken,
        httpClient: &http.Client{
            Transport: transport,
            Timeout:   30 * time.Second,
        },
        logger: logger,
    }
}

func (c *Client) apiURL(method string) string {
    return fmt.Sprintf("https://api.telegram.org/bot%s/%s", c.botToken, method)
}

// SendMessage отправляет HTML-сообщение
func (c *Client) SendMessage(chatID int64, text string) error {
    payload := map[string]interface{}{
        "chat_id":                  chatID,
        "text":                     text,
        "parse_mode":               "HTML",
        "disable_web_page_preview": true,
    }
    jsonData, err := json.Marshal(payload)
    if err != nil {
        return err
    }
    resp, err := c.httpClient.Post(c.apiURL("sendMessage"), "application/json", bytes.NewReader(jsonData))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("telegram error %d: %s", resp.StatusCode, body)
    }
    c.logger.Debug("message sent", "chat", chatID)
    return nil
}

// SendDocument отправляет файл-вложение
func (c *Client) SendDocument(chatID int64, filename string, data []byte, caption string) error {
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)

    // Поле chat_id
    if err := writer.WriteField("chat_id", strconv.FormatInt(chatID, 10)); err != nil {
        return err
    }
    if caption != "" {
        if err := writer.WriteField("caption", caption); err != nil {
            return err
        }
    }
    // Поле document
    part, err := writer.CreateFormFile("document", filename)
    if err != nil {
        return err
    }
    if _, err := part.Write(data); err != nil {
        return err
    }
    if err := writer.Close(); err != nil {
        return err
    }

    req, err := http.NewRequest("POST", c.apiURL("sendDocument"), body)
    if err != nil {
        return err
    }
    req.Header.Set("Content-Type", writer.FormDataContentType())

    resp, err := c.httpClient.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        b, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("telegram error %d: %s", resp.StatusCode, b)
    }
    c.logger.Debug("document sent", "chat", chatID, "file", filename)
    return nil
}

// GetChat получает информацию о чате
func (c *Client) GetChat(chatID int64) (*ChatInfo, error) {
    payload := map[string]interface{}{"chat_id": chatID}
    jsonData, _ := json.Marshal(payload)
    resp, err := c.httpClient.Post(c.apiURL("getChat"), "application/json", bytes.NewReader(jsonData))
    if err != nil {
        return nil, err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("telegram error %d", resp.StatusCode)
    }
    var result struct {
        Ok     bool     `json:"ok"`
        Result ChatInfo `json:"result"`
    }
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    if !result.Ok {
        return nil, fmt.Errorf("telegram returned not ok")
    }
    return &result.Result, nil
}

type ChatInfo struct {
    ID          int64  `json:"id"`
    Description string `json:"description,omitempty"`
}

// SetChatDescription устанавливает описание чата (храним last_id)
func (c *Client) SetChatDescription(chatID int64, description string) error {
    payload := map[string]interface{}{
        "chat_id":     chatID,
        "description": description,
    }
    jsonData, _ := json.Marshal(payload)
    resp, err := c.httpClient.Post(c.apiURL("setChatDescription"), "application/json", bytes.NewReader(jsonData))
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("telegram error %d: %s", resp.StatusCode, body)
    }
    c.logger.Debug("chat description updated", "chat", chatID, "desc", description)
    return nil
}