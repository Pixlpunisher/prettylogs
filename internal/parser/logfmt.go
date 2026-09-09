package parser

import (
	"strings"
	"unicode"

	"github.com/tcontardo/prettylogs/internal/record"
)

type LogfmtParser struct{}

func (LogfmtParser) Parse(line string) (record.Record, bool) {
	if !strings.Contains(line, "=") {
		return record.Record{}, false
	}
	fields, ok := parseLogfmt(line)
	if !ok || len(fields) == 0 {
		return record.Record{}, false
	}

	rec := record.Record{
		Level:   NormalizeLevel(pick(fields, "level", "lvl", "severity")),
		Message: pick(fields, "message", "msg", "log"),
		Source:  pick(fields, "source", "caller", "file", "logger", "component"),
		Raw:     line,
		Extra:   map[string]string{},
	}
	if rec.Message == "" {
		rec.Message = strings.TrimSpace(line)
	}
	if file := pick(fields, "file"); file != "" {
		if ln := pick(fields, "line"); ln != "" {
			rec.Source = file + ":" + ln
		}
	}
	if ts, ok := parseTimestamp(pick(fields, "timestamp", "time", "ts")); ok {
		rec.Timestamp = ts
	}
	used := map[string]struct{}{
		"level": {}, "lvl": {}, "severity": {},
		"message": {}, "msg": {}, "log": {},
		"source": {}, "caller": {}, "file": {}, "logger": {}, "component": {},
		"line": {}, "timestamp": {}, "time": {}, "ts": {},
	}
	for k, v := range fields {
		if _, skip := used[strings.ToLower(k)]; skip {
			continue
		}
		rec.Extra[k] = v
	}
	return rec, true
}

func parseLogfmt(line string) (map[string]string, bool) {
	fields := make(map[string]string)
	i := 0
	pairs := 0
	for i < len(line) {
		for i < len(line) && unicode.IsSpace(rune(line[i])) {
			i++
		}
		if i >= len(line) {
			break
		}
		keyStart := i
		for i < len(line) && line[i] != '=' && !unicode.IsSpace(rune(line[i])) {
			i++
		}
		if i >= len(line) || line[i] != '=' {
			return nil, false
		}
		key := line[keyStart:i]
		i++
		if key == "" {
			return nil, false
		}
		var val string
		if i < len(line) && line[i] == '"' {
			i++
			var b strings.Builder
			for i < len(line) {
				if line[i] == '\\' && i+1 < len(line) {
					b.WriteByte(line[i+1])
					i += 2
					continue
				}
				if line[i] == '"' {
					i++
					break
				}
				b.WriteByte(line[i])
				i++
			}
			val = b.String()
		} else {
			valStart := i
			for i < len(line) && !unicode.IsSpace(rune(line[i])) {
				i++
			}
			val = line[valStart:i]
		}
		fields[key] = val
		pairs++
	}
	if pairs == 0 {
		return nil, false
	}
	return fields, true
}
