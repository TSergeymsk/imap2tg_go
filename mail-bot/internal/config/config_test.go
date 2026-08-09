package config

import (
    "os"
    "testing"
)

func TestLoadValidConfig(t *testing.T) {
    // Создаём временный конфиг
    content := `{
        "imap_server": "imap.example.com:993",
        "username": "user",
        "password": "pass",
        "bot_token": "token",
        "chat_id": "123456",
        "chat_id_err": "654321",
        "encoding": "UTF-8",
        "send_attach": true,
        "proxy_url": "http://proxy:8118",
        "logfile": "test.log",
        "loglevel": "debug"
    }`
    tmpfile, err := os.CreateTemp("", "config*.json")
    if err != nil {
        t.Fatal(err)
    }
    defer os.Remove(tmpfile.Name())
    if _, err := tmpfile.Write([]byte(content)); err != nil {
        t.Fatal(err)
    }
    if err := tmpfile.Close(); err != nil {
        t.Fatal(err)
    }

    cfg, err := Load(tmpfile.Name())
    if err != nil {
        t.Fatalf("Load failed: %v", err)
    }

    if cfg.IMAPServer != "imap.example.com:993" {
        t.Errorf("IMAPServer = %s, want imap.example.com:993", cfg.IMAPServer)
    }
    if cfg.ChatID != 123456 {
        t.Errorf("ChatID = %d, want 123456", cfg.ChatID)
    }
    if cfg.ChatIDErr != 654321 {
        t.Errorf("ChatIDErr = %d, want 654321", cfg.ChatIDErr)
    }
    if cfg.LogLevel != "debug" {
        t.Errorf("LogLevel = %s, want debug", cfg.LogLevel)
    }
    if !cfg.SendAttach {
        t.Error("SendAttach should be true")
    }
}

func TestLoadConfigDefaults(t *testing.T) {
    content := `{
        "imap_server": "imap.example.com",
        "username": "user",
        "password": "pass",
        "bot_token": "token",
        "chat_id": "123456"
    }`
    tmpfile, err := os.CreateTemp("", "config*.json")
    if err != nil {
        t.Fatal(err)
    }
    defer os.Remove(tmpfile.Name())
    if _, err := tmpfile.Write([]byte(content)); err != nil {
        t.Fatal(err)
    }
    if err := tmpfile.Close(); err != nil {
        t.Fatal(err)
    }

    cfg, err := Load(tmpfile.Name())
    if err != nil {
        t.Fatalf("Load failed: %v", err)
    }

    if cfg.Encoding != "UTF-8" {
        t.Errorf("Encoding default = %s, want UTF-8", cfg.Encoding)
    }
    if cfg.LogFile != "mail-bot.log" {
        t.Errorf("LogFile default = %s, want mail-bot.log", cfg.LogFile)
    }
    if cfg.LogLevel != "info" {
        t.Errorf("LogLevel default = %s, want info", cfg.LogLevel)
    }
    if cfg.ChatIDErr != 0 {
        t.Errorf("ChatIDErr default = %d, want 0", cfg.ChatIDErr)
    }
}