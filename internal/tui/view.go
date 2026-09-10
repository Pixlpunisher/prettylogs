package tui

import (
	"fmt"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/tcontardo/prettylogs/internal/record"
)

func (m Model) View() tea.View {
	var body string
	if m.help {
		body = m.helpView()
	} else {
		body = m.mainView()
	}
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

func (m Model) mainView() string {
	var b strings.Builder
	b.WriteString(m.headerView())
	b.WriteString("\n")
	if m.filtering {
		b.WriteString(m.filterView())
		b.WriteString("\n")
	}
	b.WriteString(m.listView())
	if m.searching {
		b.WriteString("\n")
		b.WriteString(m.searchInput.View())
		if err := m.store.QueryError(); err != nil {
			b.WriteString("  ")
			b.WriteString(m.styles.SearchErr.Render(err.Error()))
		}
	}
	b.WriteString("\n")
	b.WriteString(m.footerView())
	return b.String()
}

func (m Model) headerView() string {
	counts := m.store.Counts()
	filter := m.store.Query()
	if filter == "" {
		filter = "_______"
	}
	level := m.store.Level()
	title := m.styles.Header.Render("prettylogs")
	search := fmt.Sprintf("🔍 Filter: [%s]", filter)
	stats := fmt.Sprintf("📊 ERROR %d │ WARN %d │ INFO %d │ DEBUG %d",
		counts[record.LevelError], counts[record.LevelWarn], counts[record.LevelInfo], counts[record.LevelDebug])
	meta := m.styles.Dim.Render(fmt.Sprintf("%s │ %s │ %s", search, stats, m.sourceStatus))
	if level != "All" {
		meta = m.styles.Dim.Render(fmt.Sprintf("%s │ level:%s │ %s", search, level, m.sourceStatus))
	}
	line := title + " │ " + meta
	if m.width > 0 {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(line)
	}
	return line
}

func (m Model) footerView() string {
	n := len(m.rows())
	pos := 0
	if n > 0 {
		pos = m.selected + 1
	}
	text := fmt.Sprintf("%d/%d  │  ? help  │  / search  │  l level  │  q quit  │  color:%s", pos, n, m.colorProfile)
	if err := m.store.QueryError(); err != nil {
		text = m.styles.SearchErr.Render(err.Error()) + "  │  " + text
	}
	return m.styles.Footer.Render(text)
}

func (m Model) filterView() string {
	var b strings.Builder
	b.WriteString(m.styles.Header.Render("Filter by level"))
	b.WriteString("\n")
	for i, opt := range levelOptions {
		cursor := "  "
		if i == m.filterIdx {
			cursor = "> "
		}
		line := cursor + opt
		if i == m.filterIdx {
			line = m.styles.Selected.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString(m.styles.Dim.Render("Enter to apply, Esc to cancel"))
	return b.String()
}

func (m Model) listView() string {
	rows := m.rows()
	if len(rows) == 0 {
		msg := "Waiting for logs…"
		if m.store.Len() > 0 {
			msg = "No logs match the current filter"
		}
		return m.styles.Dim.Render(msg)
	}
	start, end := m.visibleRange(rows, m.bodyHeight())
	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(m.renderRow(rows[i], i == m.selected))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) renderRow(r row, selected bool) string {
	if r.kind == rowGroup {
		return m.renderGroup(r, selected)
	}
	return m.renderEntry(r, selected)
}

func (m Model) renderGroup(r row, selected bool) string {
	first, last := r.group[0], r.group[len(r.group)-1]
	level := m.styles.Level(first.Level).Render(fmt.Sprintf("%s ×%d", levelBadge(first.Level), len(r.group)))
	header := fmt.Sprintf("%s  │ %s │ collapsed run", level, tsRange(first, last))

	var block string
	if m.expanded[r.key()] {
		lines, visible, indicator, offset := m.detailWindow(r)
		detailBlock := m.renderDetailLines(visible, m.styles.Detail)
		if indicator {
			label := scrollIndicatorText(offset, len(visible), len(lines))
			detailBlock += "\n" + strings.Repeat(" ", 4) + m.styles.Dim.Render(label)
		}
		block = header + "\n" + detailBlock
	} else {
		hint := m.styles.Dim.Render(fmt.Sprintf("⋯ %d similar rows folded — press enter to expand", len(r.group)))
		block = header + "\n" + hint
	}
	return m.wrapRow(first.Level, selected, block)
}

// wrapRow applies the level-colored left spine to a row's fully-composed
// block. Selection is shown via border weight, not color, so a row's spine
// color never changes as the cursor moves over it.
//
// lipgloss's Style.Width sets the TOTAL rendered width (it subtracts the
// style's own border+padding internally to compute the content wrap width),
// so we pass m.width directly here rather than pre-subtracting the spine's
// border(1)+padding(1) ourselves — avail (below) already budgets content to
// exactly m.width-2, matching what lipgloss will compute internally.
func (m Model) wrapRow(level string, selected bool, block string) string {
	return m.styles.Spine(level, selected).Width(max(0, m.width)).Render(block)
}

func tsRange(first, last *record.Record) string {
	fmtTS := func(t time.Time) string {
		if t.IsZero() {
			return "—"
		}
		return t.Local().Format("15:04:05")
	}
	if first.Timestamp.IsZero() && last.Timestamp.IsZero() {
		return "—"
	}
	return fmtTS(first.Timestamp) + " → " + fmtTS(last.Timestamp)
}

func (m Model) renderEntry(r row, selected bool) string {
	rec := r.rec
	ts := "—"
	if !rec.Timestamp.IsZero() {
		ts = rec.Timestamp.Local().Format("15:04:05")
	}
	src := rec.Source
	if src == "" {
		src = "—"
	}
	level := m.styles.Level(rec.Level).Render(levelBadge(rec.Level))
	prefix := fmt.Sprintf("%s │ %-8s │ %s │ ", level, ts, src)
	avail := m.width - lipgloss.Width(prefix) - 2
	msg := record.FirstLine(rec.Message)
	block := prefix + m.styledWithMatches(truncate(msg, avail), m.styles.Message)
	if m.expanded[rec.ID] {
		lines, visible, indicator, offset := m.detailWindow(r)
		detailBlock := m.renderDetailLines(visible, m.styles.Detail)
		if indicator {
			label := scrollIndicatorText(offset, len(visible), len(lines))
			detailBlock += "\n" + strings.Repeat(" ", 4) + m.styles.Dim.Render(label)
		}
		block += "\n" + detailBlock
	}
	return m.wrapRow(rec.Level, selected, block)
}

// styledWithMatches renders s with base applied to the whole string, except
// any substrings matching the active search pattern (internal/store's
// Pattern()) are rendered with Highlight instead, so matches are visible
// even when the surrounding line/detail content still shows for context.
func (m Model) styledWithMatches(s string, base lipgloss.Style) string {
	pattern := m.store.Pattern()
	if pattern == nil {
		return base.Render(s)
	}
	locs := pattern.FindAllStringIndex(s, -1)
	if len(locs) == 0 {
		return base.Render(s)
	}
	var b strings.Builder
	last := 0
	for _, loc := range locs {
		start, end := loc[0], loc[1]
		if start > last {
			b.WriteString(base.Render(s[last:start]))
		}
		if end > start {
			b.WriteString(m.styles.Highlight.Render(s[start:end]))
		}
		last = end
	}
	if last < len(s) {
		b.WriteString(base.Render(s[last:]))
	}
	return b.String()
}

// renderDetailLines styles each detail/group-member line independently (so
// styledWithMatches can highlight matches within it), then rejoins them.
func (m Model) renderDetailLines(lines []string, base lipgloss.Style) string {
	rendered := make([]string, len(lines))
	for i, l := range lines {
		rendered[i] = m.styledWithMatches(l, base)
	}
	return strings.Join(rendered, "\n")
}

func (m Model) detailText(rec *record.Record) string {
	var b strings.Builder
	raw := rec.Raw
	first := record.FirstLine(raw)
	rest := strings.TrimPrefix(raw, first)
	rest = strings.TrimPrefix(rest, "\n")
	if rest != "" {
		b.WriteString(rest)
	} else if rec.Message != record.FirstLine(rec.Message) {
		b.WriteString(strings.TrimPrefix(rec.Message, record.FirstLine(rec.Message)+"\n"))
	} else {
		b.WriteString(raw)
	}
	if len(rec.Extra) > 0 {
		if b.Len() > 0 {
			b.WriteString("\n")
		}
		keys := make([]string, 0, len(rec.Extra))
		for k := range rec.Extra {
			keys = append(keys, k)
		}
		for i, k := range keys {
			if i > 0 {
				b.WriteString("\n")
			}
			b.WriteString(fmt.Sprintf("%s: %s", k, rec.Extra[k]))
		}
	}
	return b.String()
}

func (m Model) helpView() string {
	box := m.styles.Help.
		Border(lipgloss.NormalBorder()).
		BorderForeground(themeColor("dim")).
		Width(max(40, min(m.width-4, 64))).
		Render(helpText + "\nPress ? or Esc to close")
	return box
}

func truncate(s string, width int) string {
	if width <= 0 || lipgloss.Width(s) <= width {
		return s
	}
	runes := []rune(s)
	if len(runes) <= 1 {
		return s
	}
	for len(runes) > 1 && lipgloss.Width(string(runes)+"…") > width {
		runes = runes[:len(runes)-1]
	}
	return string(runes) + "…"
}
