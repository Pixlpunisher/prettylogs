package parser

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/tcontardo/prettylogs/internal/record"
)

type SyslogParser struct{}

var (
	rfc5424 = regexp.MustCompile(`^<(\d+)>\d+\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(\S+)\s+(-|\[.*?\])\s+(.*)$`)
	rfc3164 = regexp.MustCompile(`^<(\d+)>([A-Z][a-z]{2}\s+\d{1,2}\s+\d{2}:\d{2}:\d{2})\s+(\S+)\s+([^:]+):\s*(.*)$`)
)

func (SyslogParser) Parse(line string) (record.Record, bool) {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, "<") {
		return record.Record{}, false
	}
	if m := rfc5424.FindStringSubmatch(trimmed); m != nil {
		pri, _ := strconv.Atoi(m[1])
		rec := record.Record{
			Level:   syslogSeverityLevel(pri % 8),
			Source:  m[4],
			Message: m[8],
			Raw:     line,
			Extra:   map[string]string{"hostname": m[3], "procid": m[5]},
		}
		if ts, ok := parseTimestamp(m[2]); ok {
			rec.Timestamp = ts
		}
		if rec.Source == "-" {
			rec.Source = m[3]
		}
		return rec, true
	}
	if m := rfc3164.FindStringSubmatch(trimmed); m != nil {
		pri, _ := strconv.Atoi(m[1])
		rec := record.Record{
			Level:   syslogSeverityLevel(pri % 8),
			Source:  strings.TrimSpace(m[4]),
			Message: m[5],
			Raw:     line,
			Extra:   map[string]string{"hostname": m[3]},
		}
		if ts, ok := parseTimestamp(m[2]); ok {
			rec.Timestamp = ts
		}
		return rec, true
	}
	return record.Record{}, false
}
