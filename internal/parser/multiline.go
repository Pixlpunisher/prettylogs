package parser

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/charmbracelet/x/ansi"
	"github.com/tcontardo/prettylogs/internal/record"
)

var logTimeLevel = regexp.MustCompile(`(?i)^\s*\d{2}:\d{2}:\d{2}(?:\.\d+)?Z?\s+(?:\[.*?\]\s+)?(ERROR|ERR|WARN(?:ING)?|INFO|DEBUG|TRACE|FATAL|PANIC|CRITICAL|CRIT|SEVERE)\b`)

// IsContinuation reports whether line continues the pending record at
// prevLevel rather than starting a new independent log entry.
// A new log header always starts a new record. Separator-only lines join any
// previous record. Unprefixed body after ERROR/WARN joins that record so a
// Spring failure analysis stays one block. Stack frames still join.
// Maven reactor noise and help [ERROR] lines join the previous ERROR/WARN.
// An Nx epilogue starts a new record unless the pending raw is already Nx.
func IsContinuation(prevLevel, prevRaw, line string) bool {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" {
		return false
	}
	if isNxEpilogue(trimmed) {
		return isNxPending(prevRaw)
	}
	if isErrorOrWarn(prevLevel) && (isMavenReactorNoise(trimmed) || isMavenHelp(trimmed)) {
		return true
	}
	if looksLikeNewRecord(line) {
		return false
	}
	if strings.HasPrefix(trimmed, "at ") || strings.HasPrefix(trimmed, "Caused by:") || strings.HasPrefix(trimmed, "... ") {
		return true
	}
	if isSeparator(trimmed) {
		return true
	}
	return isErrorOrWarn(prevLevel)
}

func isErrorOrWarn(level string) bool {
	return level == record.LevelError || level == record.LevelWarn
}

func isNxPending(prevRaw string) bool {
	return isNxEpilogue(record.FirstLine(prevRaw))
}

func looksLikeNewRecord(line string) bool {
	trimmed := strings.TrimSpace(line)
	if strings.HasPrefix(trimmed, "{") {
		return true
	}
	if levelPrefix.MatchString(trimmed) {
		return true
	}
	if leadingTime.MatchString(trimmed) {
		return true
	}
	return logTimeLevel.MatchString(trimmed)
}

func mavenPayload(trimmed, wantLevel string) (payload string, ok bool) {
	m := levelPrefix.FindStringSubmatch(trimmed)
	if m == nil {
		return "", false
	}
	if NormalizeLevel(m[1]) != wantLevel {
		return "", false
	}
	return strings.TrimSpace(m[2]), true
}

func isMavenReactorNoise(trimmed string) bool {
	payload, tagged := mavenPayload(trimmed, record.LevelInfo)
	if !tagged {
		if levelPrefix.MatchString(trimmed) {
			return false
		}
		payload = trimmed
	}
	if payload == "" || isSeparator(payload) {
		return true
	}
	lower := strings.ToLower(payload)
	switch {
	case lower == "build failure", lower == "build success":
		return true
	case strings.HasPrefix(lower, "total time:"), strings.HasPrefix(lower, "finished at:"):
		return true
	default:
		return false
	}
}

func isMavenHelp(trimmed string) bool {
	if strings.HasPrefix(trimmed, "[Help ") {
		return true
	}
	payload, ok := mavenPayload(trimmed, record.LevelError)
	if !ok {
		return false
	}
	if payload == "" {
		return true
	}
	lower := strings.ToLower(payload)
	if strings.Contains(lower, "failed to execute goal") {
		return false
	}
	switch {
	case strings.Contains(lower, "to see the full stack trace"),
		strings.Contains(lower, "re-run maven"),
		strings.Contains(lower, "for more information about the errors"),
		strings.HasPrefix(lower, "[help "):
		return true
	default:
		return false
	}
}

func isNxEpilogue(trimmed string) bool {
	if strings.Contains(trimmed, "NX") && (strings.Contains(trimmed, "Running target") ||
		strings.Contains(trimmed, "flaky") || strings.Contains(trimmed, "Nx detected")) {
		return true
	}
	return strings.HasPrefix(trimmed, "Failed tasks:") || strings.HasPrefix(trimmed, "Hint: run the command")
}

func isSeparator(trimmed string) bool {
	if len(trimmed) < 3 {
		return false
	}
	for _, r := range trimmed {
		if !isSeparatorRune(r) {
			return false
		}
	}
	return true
}

func isSeparatorRune(r rune) bool {
	switch r {
	case '*', '=', '-', '#', '─', '━', '═', '▬', '_':
		return true
	default:
		return unicode.IsSpace(r)
	}
}

type Assembler struct {
	parser  Parser
	pending *record.Record
}

func NewAssembler(p Parser) *Assembler {
	return &Assembler{parser: p}
}

func (a *Assembler) Feed(line string) []record.Record {
	line = ansi.Strip(line)
	if strings.TrimSpace(line) == "" {
		return nil
	}
	if a.pending != nil && IsContinuation(a.pending.Level, a.pending.Raw, line) {
		a.pending.Raw += "\n" + line
		if a.pending.Message != "" {
			a.pending.Message += "\n" + line
		} else {
			a.pending.Message = line
		}
		return nil
	}
	var out []record.Record
	if a.pending != nil {
		out = append(out, *a.pending)
		a.pending = nil
	}
	rec := ParseLine(a.parser, line)
	a.pending = &rec
	return out
}

func (a *Assembler) Flush() []record.Record {
	if a.pending == nil {
		return nil
	}
	rec := *a.pending
	a.pending = nil
	return []record.Record{rec}
}
