package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsLogFile(t *testing.T) {
	dir := t.TempDir()
	logPath := filepath.Join(dir, "app.log")
	scriptPath := filepath.Join(dir, "start.sh")
	if err := os.WriteFile(logPath, []byte("INFO hi\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(scriptPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		arg  string
		want bool
	}{{logPath, true}, {scriptPath, false}, {dir, false}, {"npm", false}} {
		if got := isLogFile(tc.arg); got != tc.want {
			t.Errorf("isLogFile(%q) = %v, want %v", tc.arg, got, tc.want)
		}
	}
}
