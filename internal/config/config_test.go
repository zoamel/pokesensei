package config

import (
	"log/slog"
	"testing"
)

func TestLoadFromEnv_AllSet(t *testing.T) {
	t.Setenv("DATABASE_PATH", "data/test.db")
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabasePath != "data/test.db" {
		t.Errorf("DatabasePath = %q, want %q", cfg.DatabasePath, "data/test.db")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelDebug)
	}
}

func TestLoadFromEnv_Defaults(t *testing.T) {
	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabasePath != "data/pokesensei.db" {
		t.Errorf("DatabasePath = %q, want default %q", cfg.DatabasePath, "data/pokesensei.db")
	}
	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default %q", cfg.Port, "8080")
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want default %v", cfg.LogLevel, slog.LevelInfo)
	}
}
