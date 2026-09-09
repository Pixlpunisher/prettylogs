package parser

import (
	"encoding/json"
	"strings"

	"github.com/tcontardo/prettylogs/internal/record"
)

type JSONParser struct{}

func (JSONParser) Parse(line string) (record.Record, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "{") {
		return record.Record{}, false
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(trimmed), &raw); err != nil {
		return record.Record{}, false
	}

	fields := make(map[string]string, len(raw))
	for k, v := range raw {
		if s := stringify(v); s != "" {
			fields[k] = s
		}
	}

	rec := record.Record{
		Level:   NormalizeLevel(pick(fields, "level", "lvl", "severity", "log.level")),
		Message: pick(fields, "message", "msg", "log", "text"),
		Source:  pick(fields, "source", "caller", "file", "logger", "component", "logger_name"),
		Raw:     line,
		Extra:   map[string]string{},
	}
	if rec.Message == "" {
		rec.Message = trimmed
	}
	if file := pick(fields, "file"); file != "" {
		if ln := pick(fields, "line"); ln != "" {
			rec.Source = file + ":" + ln
		}
	}
	if ts, ok := parseTimestamp(pick(fields, "timestamp", "time", "ts", "datetime", "@timestamp")); ok {
		rec.Timestamp = ts
	}
	used := map[string]struct{}{
		"level": {}, "lvl": {}, "severity": {}, "log.level": {},
		"message": {}, "msg": {}, "log": {}, "text": {},
		"source": {}, "caller": {}, "file": {}, "logger": {}, "component": {}, "logger_name": {},
		"line": {}, "timestamp": {}, "time": {}, "ts": {}, "datetime": {}, "@timestamp": {},
	}
	for k, v := range fields {
		if _, skip := used[strings.ToLower(k)]; skip {
			continue
		}
		rec.Extra[k] = v
	}
	return rec, true
}
