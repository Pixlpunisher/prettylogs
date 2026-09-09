package parser

import (
	"fmt"
	"strings"

	"github.com/tcontardo/prettylogs/internal/record"
)

type Parser interface {
	Parse(line string) (record.Record, bool)
}

type AutoParser struct{}

func ForFormat(format string) (Parser, error) {
	switch strings.ToLower(strings.TrimSpace(format)) {
	case "", "auto":
		return AutoParser{}, nil
	case "json":
		return JSONParser{}, nil
	case "logfmt":
		return LogfmtParser{}, nil
	case "syslog":
		return SyslogParser{}, nil
	case "plain":
		return PlainParser{}, nil
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
}

func (AutoParser) Parse(line string) (record.Record, bool) {
	if rec, ok := (JSONParser{}).Parse(line); ok {
		return rec, true
	}
	if rec, ok := (LogfmtParser{}).Parse(line); ok {
		return rec, true
	}
	if rec, ok := (SyslogParser{}).Parse(line); ok {
		return rec, true
	}
	return (PlainParser{}).Parse(line)
}

func ParseLine(p Parser, line string) record.Record {
	if p == nil {
		p = AutoParser{}
	}
	rec, ok := p.Parse(line)
	if !ok {
		rec = parsePlain(line)
	}
	if rec.Raw == "" {
		rec.Raw = line
	}
	if rec.Level == "" {
		rec.Level = record.LevelInfo
	}
	return rec
}
