package tui

import (
	"fmt"
	"strings"

	"github.com/tcontardo/prettylogs/internal/record"
)

// detailCapacity returns how many detail-content lines can be shown at once
// for an expanded row: bodyHeight minus the 1 line its own header/dense
// line always consumes.
func (m Model) detailCapacity() int {
	c := m.bodyHeight() - 1
	if c < 1 {
		c = 1
	}
	return c
}

// detailLines returns the full, unwindowed detail-content lines for row r:
// one compact line per member for a rowGroup (identical text to
// renderGroup's existing expanded member lines), or detailText() split on
// newlines for a rowRecord.
func (m Model) detailLines(r row) []string {
	if r.kind == rowGroup {
		lines := make([]string, len(r.group))
		for i, rec := range r.group {
			ts := "—"
			if !rec.Timestamp.IsZero() {
				ts = rec.Timestamp.Local().Format("15:04:05")
			}
			lines[i] = fmt.Sprintf("%s  %s", ts, truncate(record.FirstLine(rec.Message), m.width-8))
		}
		return lines
	}
	if r.rec == nil {
		return nil
	}
	return strings.Split(m.detailText(r.rec), "\n")
}

// detailWindow computes, for row r's expanded detail: the full line slice,
// the slice visible at the current clamped offset, whether a scroll
// indicator is needed, and the offset used. Single source of truth for
// expanded-row sizing — entryHeight and the render functions both go
// through this so they can't drift apart.
func (m Model) detailWindow(r row) (lines, visible []string, indicator bool, offset int) {
	lines = m.detailLines(r)
	total := len(lines)
	capacity := m.detailCapacity()
	if total <= capacity {
		return lines, lines, false, 0
	}
	visibleN := capacity - 1
	if visibleN < 1 {
		visibleN = 1
	}
	if m.detailScrollKey == r.key() {
		offset = m.detailScrollOffset
	}
	maxOffset := total - visibleN
	if maxOffset < 0 {
		maxOffset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	if offset < 0 {
		offset = 0
	}
	return lines, lines[offset : offset+visibleN], true, offset
}

// detailScrollActive reports whether row r is expanded AND its content
// needs scrolling. Only then do j/k-family keys scroll detail instead of
// moving row selection.
func (m Model) detailScrollActive(r row) bool {
	if !m.expanded[r.key()] {
		return false
	}
	_, _, indicator, _ := m.detailWindow(r)
	return indicator
}

// scrollDetail adjusts row r's stored scroll offset by delta lines, clamped
// to [0, maxOffset], and records which row it applies to.
func (m *Model) scrollDetail(r row, delta int) {
	lines, visible, _, offset := m.detailWindow(r)
	maxOffset := len(lines) - len(visible)
	if maxOffset < 0 {
		maxOffset = 0
	}
	offset += delta
	if offset < 0 {
		offset = 0
	}
	if offset > maxOffset {
		offset = maxOffset
	}
	m.detailScrollKey = r.key()
	m.detailScrollOffset = offset
}

// scrollIndicatorText formats the 1-based inclusive visible line range.
func scrollIndicatorText(offset, visibleCount, total int) string {
	first := offset + 1
	last := offset + visibleCount
	return fmt.Sprintf("── lines %d-%d of %d · j/k scroll, enter to collapse ──", first, last, total)
}
