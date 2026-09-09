package parser

import (
	"strconv"
	"strings"
	"time"
)

var timeLayouts = []string{
	time.RFC3339Nano,
	time.RFC3339,
	"2006-01-02 15:04:05.000",
	"2006-01-02 15:04:05",
	"2006-01-02T15:04:05.000Z07:00",
	"2006/01/02 15:04:05",
	"Jan 2 15:04:05",
	"Jan  2 15:04:05",
}

func parseTimestamp(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	if unix, err := strconv.ParseFloat(s, 64); err == nil && unix > 1e9 {
		sec := int64(unix)
		nsec := int64((unix - float64(sec)) * 1e9)
		return time.Unix(sec, nsec), true
	}
	for _, layout := range timeLayouts {
		if ts, err := time.Parse(layout, s); err == nil {
			return ts, true
		}
	}
	return time.Time{}, false
}

func pick(m map[string]string, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != "" {
			return v
		}
		for mk, v := range m {
			if strings.EqualFold(mk, k) && v != "" {
				return v
			}
		}
	}
	return ""
}

func stringify(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return ""
	}
}
