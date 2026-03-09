package config

import (
	"log/slog"
	"os"
)

type Config struct {
	DatabasePath string
	Port         string
	LogLevel     slog.Level
}

func LoadFromEnv() (*Config, error) {
	dbPath := os.Getenv("DATABASE_PATH")
	if dbPath == "" {
		dbPath = "data/pokesensei.db"
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		DatabasePath: dbPath,
		Port:         port,
		LogLevel:     parseLogLevel(os.Getenv("LOG_LEVEL")),
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
