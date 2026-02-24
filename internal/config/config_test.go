package config

import (
	"log/slog"
	"testing"
)

func TestLoadFromEnv_AllSet(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/testdb")
	t.Setenv("PORT", "9090")
	t.Setenv("LOG_LEVEL", "debug")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.DatabaseURL != "postgres://test:test@localhost/testdb" {
		t.Errorf("DatabaseURL = %q, want %q", cfg.DatabaseURL, "postgres://test:test@localhost/testdb")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
	if cfg.LogLevel != slog.LevelDebug {
		t.Errorf("LogLevel = %v, want %v", cfg.LogLevel, slog.LevelDebug)
	}
}

func TestLoadFromEnv_Defaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://test:test@localhost/testdb")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Port != "8080" {
		t.Errorf("Port = %q, want default %q", cfg.Port, "8080")
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want default %v", cfg.LogLevel, slog.LevelInfo)
	}
}

func TestLoadFromEnv_MissingDatabaseURL(t *testing.T) {
	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL, got nil")
	}
}
