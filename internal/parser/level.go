package parser

import (
	"strings"

	"github.com/tcontardo/prettylogs/internal/record"
)

func NormalizeLevel(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "error", "err", "fatal", "panic", "severe", "crit", "critical", "emerg", "alert":
		return record.LevelError
	case "warn", "warning":
		return record.LevelWarn
	case "debug", "trace", "verbose", "dbg":
		return record.LevelDebug
	default:
		return record.LevelInfo
	}
}

func syslogSeverityLevel(severity int) string {
	switch {
	case severity <= 3:
		return record.LevelError
	case severity == 4:
		return record.LevelWarn
	case severity == 7:
		return record.LevelDebug
	default:
		return record.LevelInfo
	}
}
