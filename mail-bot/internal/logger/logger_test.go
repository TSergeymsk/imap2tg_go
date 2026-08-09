package logger

import (
    "os"
    "testing"
)

func TestSetupLogger(t *testing.T) {
    logFile := "test.log"
    defer os.Remove(logFile)

    log := Setup(logFile, "debug")
    if log == nil {
        t.Fatal("Setup returned nil")
    }
    // Проверяем, что файл создался
    if _, err := os.Stat(logFile); os.IsNotExist(err) {
        t.Errorf("Log file %s not created", logFile)
    }
}