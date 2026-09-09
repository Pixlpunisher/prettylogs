package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Parallel()
	cfg := Load(filepath.Join(t.TempDir(), "does-not-exist.yaml"))
	if cfg.Theme.Error != "red" || cfg.Theme.Warn != "yellow" {
		t.Fatalf("defaults %+v", cfg.Theme)
	}
}

func TestLoadReadsTheme(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "theme.yaml")
	content := []byte("theme:\n  error: magenta\n  warn: cyan\n")
	if err := os.WriteFile(path, content, 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := Load(path)
	if cfg.Theme.Error != "magenta" {
		t.Fatalf("error color %q", cfg.Theme.Error)
	}
	if cfg.Theme.Warn != "cyan" {
		t.Fatalf("warn color %q", cfg.Theme.Warn)
	}
	if cfg.Theme.Info != "blue" {
		t.Fatalf("info should keep default, got %q", cfg.Theme.Info)
	}
}
