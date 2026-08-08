package config

import (
    "encoding/json"
    "os"
)

type Config struct {
    IMAPServer string `json:"imap_server"`
    Username   string `json:"username"`
    Password   string `json:"password"`
    BotToken   string `json:"bot_token"`
    ChatID     int64  `json:"chat_id,string"`
    ChatIDErr  int64  `json:"chat_id_err,string,omitempty"`
    Encoding   string `json:"encoding"`
    SendAttach bool   `json:"send_attach"`
    ProxyURL   string `json:"proxy_url,omitempty"`
    LogFile    string `json:"logfile"`
    LogLevel   string `json:"loglevel"`
}

func Load(path string) (*Config, error) {
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    var cfg Config
    if err := json.Unmarshal(data, &cfg); err != nil {
        return nil, err
    }
    if cfg.Encoding == "" {
        cfg.Encoding = "UTF-8"
    }
    if cfg.LogFile == "" {
        cfg.LogFile = "mail-bot.log"
    }
    if cfg.LogLevel == "" {
        cfg.LogLevel = "info"
    }
    return &cfg, nil
}