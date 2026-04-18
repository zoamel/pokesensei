package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveSeedFile_ReturnsPathWhenFileExists(t *testing.T) {
	dir := t.TempDir()
	seedDir := filepath.Join(dir, "db", "seed")
	if err := os.MkdirAll(seedDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(seedDir, "xy_trainers.json")
	if err := os.WriteFile(target, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	path, ok := resolveSeedFile("xy")
	if !ok {
		t.Fatal("expected ok=true for existing file")
	}
	if path != "db/seed/xy_trainers.json" {
		t.Errorf("path = %q, want db/seed/xy_trainers.json", path)
	}
}

func TestResolveSeedFile_ReturnsFalseWhenMissing(t *testing.T) {
	dir := t.TempDir()
	oldWd, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWd) })
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}

	_, ok := resolveSeedFile("nonexistent")
	if ok {
		t.Error("expected ok=false for missing file")
	}
}
