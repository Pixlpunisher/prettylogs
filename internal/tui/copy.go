package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/tcontardo/prettylogs/internal/record"
)

const (
	libraryRunMin       = 3
	prettyBodyMaxLines  = 40
	prettyBodyHeadLines = 15
	prettyBodyTailLines = 8
)

// writeClipboard is clipboard.WriteAll in production; tests replace it so we
// never need a real pasteboard.
var writeClipboard = clipboard.WriteAll

// formatCopy is the clipboard payload for a list row. raw true is the original
// log line(s); raw false is a paste-friendly summary. Expanded vs collapsed does
// not matter — we copy the underlying records, not the current view.
func formatCopy(r row, raw bool) string {
	recs := copyRecords(r)
	if len(recs) == 0 {
		return ""
	}
	if raw {
		lines := make([]string, len(recs))
		for i, rec := range recs {
			lines[i] = rec.Raw
		}
		return strings.Join(lines, "\n")
	}
	return formatPretty(recs)
}

// copyRecords flattens a row to the records it represents. A group is every
// member in store order, not just the first — otherwise y on a folded INFO×12
// would paste a single useless summary line.
func copyRecords(r row) []*record.Record {
	switch r.kind {
	case rowRecord:
		if r.rec == nil {
			return nil
		}
		return []*record.Record{r.rec}
	case rowGroup:
		if len(r.group) == 0 {
			return nil
		}
		return r.group
	default:
		return nil
	}
}

type prettyStats struct {
	kept  int
	total int
}

func (s prettyStats) omitted() bool {
	return s.total > s.kept
}

func addPrettyStats(a, b prettyStats) prettyStats {
	return prettyStats{kept: a.kept + b.kept, total: a.total + b.total}
}

func formatPretty(recs []*record.Record) string {
	text, _ := formatPrettyWithStats(recs)
	return text
}

func formatPrettyWithStats(recs []*record.Record) (string, prettyStats) {
	if len(recs) == 1 {
		return formatPrettyOneStats(recs[0])
	}
	return formatPrettyGroup(recs), prettyStats{}
}

func formatPrettyOneStats(rec *record.Record) (string, prettyStats) {
	// Header is "LEVEL[  time][  source]" — omit empty fields rather than
	// printing the TUI's "—" placeholders, which are noise in a Slack paste.
	parts := []string{rec.Level}
	if ts := formatTS(rec.Timestamp); ts != "" {
		parts = append(parts, ts)
	}
	if rec.Source != "" {
		parts = append(parts, rec.Source)
	}

	body, stats := collapsePrettyBody(rec.Message)
	extras, extraStats := formatPrettyExtras(rec.Message, rec.Extra)
	stats = addPrettyStats(stats, extraStats)

	var b strings.Builder
	b.WriteString(strings.Join(parts, "  "))
	if body != "" {
		b.WriteByte('\n')
		b.WriteString(body)
	}
	if extras != "" {
		b.WriteString("\n\n")
		b.WriteString(extras)
	}
	return b.String(), stats
}

func formatPrettyExtras(message string, extra map[string]string) (string, prettyStats) {
	if len(extra) == 0 {
		return "", prettyStats{}
	}
	keys := make([]string, 0, len(extra))
	for k, v := range extra {
		if isStackExtraKey(k) && v != "" && strings.Contains(message, v) {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	if len(keys) == 0 {
		return "", prettyStats{}
	}
	var b strings.Builder
	var stats prettyStats
	for i, k := range keys {
		if i > 0 {
			b.WriteByte('\n')
		}
		v := extra[k]
		if strings.Contains(v, "\n") {
			collapsed, st := collapsePrettyBody(v)
			stats = addPrettyStats(stats, st)
			b.WriteString(k)
			b.WriteString(":\n")
			b.WriteString(collapsed)
			continue
		}
		b.WriteString(fmt.Sprintf("%s: %s", k, v))
		stats.kept++
		stats.total++
	}
	return b.String(), stats
}

func isStackExtraKey(k string) bool {
	switch strings.ToLower(k) {
	case "stack", "err", "error", "exception", "stacktrace", "stack_trace":
		return true
	default:
		return false
	}
}

func collapsePrettyBody(text string) (string, prettyStats) {
	if text == "" {
		return "", prettyStats{}
	}
	lines := strings.Split(text, "\n")
	total := len(lines)
	lines = collapseLibraryRuns(lines)
	lines = collapseHeadTail(lines)
	return strings.Join(lines, "\n"), prettyStats{kept: len(lines), total: total}
}

func collapseLibraryRuns(lines []string) []string {
	out := make([]string, 0, len(lines))
	var run []string
	flush := func() {
		if len(run) >= libraryRunMin {
			out = append(out, fmt.Sprintf("    … %d library frames omitted …", len(run)))
		} else {
			out = append(out, run...)
		}
		run = run[:0]
	}
	for _, line := range lines {
		if isStackFrame(line) && isLibraryFrame(line) {
			run = append(run, line)
			continue
		}
		flush()
		out = append(out, line)
	}
	flush()
	return out
}

func collapseHeadTail(lines []string) []string {
	if len(lines) <= prettyBodyMaxLines {
		return lines
	}
	omitted := len(lines) - prettyBodyHeadLines - prettyBodyTailLines
	out := make([]string, 0, prettyBodyHeadLines+prettyBodyTailLines+1)
	out = append(out, lines[:prettyBodyHeadLines]...)
	out = append(out, fmt.Sprintf("… %d lines omitted …", omitted))
	out = append(out, lines[len(lines)-prettyBodyTailLines:]...)
	return out
}

func isStackFrame(line string) bool {
	t := strings.TrimSpace(line)
	switch {
	case strings.HasPrefix(t, "at "):
		return true
	case strings.HasPrefix(t, `File "`):
		return true
	case strings.HasPrefix(t, "...") && strings.Contains(t, "more"):
		return true
	default:
		return false
	}
}

func isLibraryFrame(line string) bool {
	for _, marker := range libraryFrameMarkers {
		if strings.Contains(line, marker) {
			return true
		}
	}
	return false
}

var libraryFrameMarkers = []string{
	"node_modules",
	"node:",
	"site-packages",
	"org.springframework",
	"org.apache.",
	"java.base",
	"jdk.internal",
	"sun.",
	"net.sf.cglib",
	"io.undertow",
}

func formatPrettyGroup(recs []*record.Record) string {
	first, last := recs[0], recs[len(recs)-1]
	header := fmt.Sprintf("%s ×%d", first.Level, len(recs))
	if rng := copyTSRange(first, last); rng != "" {
		header += "  " + rng
	}
	lines := make([]string, 0, len(recs)+2)
	lines = append(lines, header, "")
	for _, rec := range recs {
		ts := formatTS(rec.Timestamp)
		if ts == "" {
			ts = "—"
		}
		lines = append(lines, ts+"  "+rec.Message)
	}
	return strings.Join(lines, "\n")
}

// copyTSRange is like tsRange in view.go, but returns "" when neither timestamp
// is set so the group header does not become "INFO ×12  —".
func copyTSRange(first, last *record.Record) string {
	if first.Timestamp.IsZero() && last.Timestamp.IsZero() {
		return ""
	}
	a, b := formatTS(first.Timestamp), formatTS(last.Timestamp)
	if a == "" {
		a = "—"
	}
	if b == "" {
		b = "—"
	}
	return a + " → " + b
}

func formatTS(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	// Local matches the TUI list; UTC in a paste would disagree with what you saw.
	return t.Local().Format("15:04:05")
}

// copySelection writes the selected row to the clipboard and sets copyStatus
// for the footer. It never quits the TUI on failure.
func (m *Model) copySelection(raw bool) {
	r, ok := m.currentRow()
	if !ok {
		m.copyStatus = "nothing to copy"
		return
	}
	recs := copyRecords(r)
	if len(recs) == 0 {
		m.copyStatus = "nothing to copy"
		return
	}
	var text string
	var stats prettyStats
	if raw {
		text = formatCopy(r, true)
	} else {
		text, stats = formatPrettyWithStats(recs)
	}
	if err := writeClipboard(text); err != nil {
		m.copyStatus = "copy failed: " + err.Error()
		return
	}
	noun := "record"
	if len(recs) != 1 {
		noun = "records"
	}
	if raw {
		m.copyStatus = fmt.Sprintf("copied %d %s (raw)", len(recs), noun)
		return
	}
	if len(recs) == 1 && stats.omitted() {
		m.copyStatus = fmt.Sprintf("copied 1 record (%d of %d lines)", stats.kept, stats.total)
		return
	}
	m.copyStatus = fmt.Sprintf("copied %d %s", len(recs), noun)
}
