package parser

import (
	"regexp"
	"strings"

	"github.com/tcontardo/prettylogs/internal/record"
)

type PlainParser struct{}

var (
	levelPrefix = regexp.MustCompile(`(?i)^\s*\[?(ERROR|ERR|WARN(?:ING)?|INFO|DEBUG|TRACE|FATAL|PANIC|CRITICAL|CRIT|SEVERE)\]?[:\s-]+\s*(.*)$`)
	leadingTime = regexp.MustCompile(`^(\d{4}-\d{2}-\d{2}[ T]\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?)\s+(.*)$`)
	sourceRef   = regexp.MustCompile(`([\w./\\-]+\.\w+:\d+)`)
)

func (PlainParser) Parse(line string) (record.Record, bool) {
	return parsePlain(line), true
}

func parsePlain(line string) record.Record {
	rec := record.Record{
		Level:   record.LevelInfo,
		Message: strings.TrimSpace(line),
		Raw:     line,
		Extra:   map[string]string{},
	}
	rest := strings.TrimSpace(line)
	if m := leadingTime.FindStringSubmatch(rest); m != nil {
		if ts, ok := parseTimestamp(m[1]); ok {
			rec.Timestamp = ts
		}
		rest = m[2]
		rec.Message = rest
	}
	if m := levelPrefix.FindStringSubmatch(rest); m != nil {
		rec.Level = NormalizeLevel(m[1])
		rec.Message = strings.TrimSpace(m[2])
	} else {
		rec.Level = inferFrontendLevel(rest)
	}
	if src := sourceRef.FindString(line); src != "" {
		rec.Source = src
	}
	if rec.Message == "" {
		rec.Message = strings.TrimSpace(line)
	}
	return rec
}

func inferFrontendLevel(line string) string {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "error"), strings.Contains(lower, "failed"), strings.Contains(lower, "fatal"),
		strings.Contains(lower, "panic"), strings.Contains(lower, "critical"):
		if strings.Contains(lower, "compiled successfully") {
			return record.LevelInfo
		}
		return record.LevelError
	case strings.HasPrefix(line, "ready -"), strings.HasPrefix(line, "event -"):
		return record.LevelInfo
	case strings.Contains(line, "[vite]"):
		return record.LevelInfo
	case strings.HasPrefix(line, "Compiled"), strings.HasPrefix(line, "Compiling"):
		return record.LevelInfo
	case strings.Contains(lower, "warn"):
		return record.LevelWarn
	default:
		return record.LevelInfo
	}
}

// CompilePhase reports whether line starts or ends a compile cycle.
// End is checked first so "Compiled successfully" is not treated as a start.
func CompilePhase(line string) (start, end bool) {
	lower := strings.ToLower(line)
	switch {
	case strings.Contains(lower, "compiled") && strings.Contains(lower, "successfully"),
		strings.Contains(lower, "ready - started server"),
		strings.Contains(lower, "listening on"),
		strings.Contains(lower, "local: http://"):
		return false, true
	case strings.Contains(lower, "compiling"),
		strings.Contains(lower, "starting compilation"),
		strings.Contains(lower, "rebuild"):
		return true, false
	default:
		return false, false
	}
}
