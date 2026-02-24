package config

import (
	"fmt"
	"log/slog"
	"os"
)

type Config struct {
	DatabaseURL string
	Port        string
	LogLevel    slog.Level
}

func LoadFromEnv() (*Config, error) {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		DatabaseURL: dbURL,
		Port:        port,
		LogLevel:    parseLogLevel(os.Getenv("LOG_LEVEL")),
	}, nil
}

func parseLogLevel(s string) slog.Level {
	switch s {
	case "debug":
		return slog.LevelDebug
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
