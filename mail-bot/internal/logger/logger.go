package logger

import (
    "io"
    "log/slog"
    "os"
    "strings"
)

func Setup(logFile, level string) *slog.Logger {
    var logLevel slog.Level
    switch strings.ToLower(level) {
    case "debug":
        logLevel = slog.LevelDebug
    case "warn", "warning":
        logLevel = slog.LevelWarn
    case "error":
        logLevel = slog.LevelError
    default:
        logLevel = slog.LevelInfo
    }

    // Файл для логов
    file, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
    if err != nil {
        // Если файл не открыть, пишем только в stdout
        file = os.Stdout
    }
    multi := io.MultiWriter(file, os.Stdout)

    handler := slog.NewJSONHandler(multi, &slog.HandlerOptions{
        Level: logLevel,
    })
    return slog.New(handler)
}