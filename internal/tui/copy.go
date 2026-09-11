package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/atotto/clipboard"
	"github.com/tcontardo/prettylogs/internal/record"
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

func formatPretty(recs []*record.Record) string {
	if len(recs) == 1 {
		return formatPrettyOne(recs[0])
	}
	return formatPrettyGroup(recs)
}

func formatPrettyOne(rec *record.Record) string {
	// Header is "LEVEL[  time][  source]" — omit empty fields rather than
	// printing the TUI's "—" placeholders, which are noise in a Slack paste.
	parts := []string{rec.Level}
	if ts := formatTS(rec.Timestamp); ts != "" {
		parts = append(parts, ts)
	}
	if rec.Source != "" {
		parts = append(parts, rec.Source)
	}
	var b strings.Builder
	b.WriteString(strings.Join(parts, "  "))
	b.WriteByte('\n')
	// Full message, not FirstLine: stack traces are why you copy an ERROR.
	b.WriteString(rec.Message)
	if len(rec.Extra) > 0 {
		keys := make([]string, 0, len(rec.Extra))
		for k := range rec.Extra {
			keys = append(keys, k)
		}
		sort.Strings(keys) // map iteration order is random; sorted extras are testable and readable
		b.WriteByte('\n')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte('\n')
			}
			b.WriteString(fmt.Sprintf("%s: %s", k, rec.Extra[k]))
		}
	}
	return b.String()
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
	if err := writeClipboard(formatCopy(r, raw)); err != nil {
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
	m.copyStatus = fmt.Sprintf("copied %d %s", len(recs), noun)
}
