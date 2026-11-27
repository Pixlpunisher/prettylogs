package record

import "time"

const (
	LevelError = "ERROR"
	LevelWarn  = "WARN"
	LevelInfo  = "INFO"
	LevelDebug = "DEBUG"
)

type Record struct {
	ID        uint64
	Level     string
	Timestamp time.Time
	Message   string
	Source    string
	Raw       string
	Extra     map[string]string

	// RestartMarker records carry no log line: they mark where a restarted command's output begins.
	RestartMarker bool
}

func FirstLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			return s[:i]
		}
	}
	return s
}
