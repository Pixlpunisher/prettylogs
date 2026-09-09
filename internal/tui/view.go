package tui

import (
	"fmt"
	"strings"

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
	n := len(m.store.Filtered())
	pos := 0
	if n > 0 {
		pos = m.selected + 1
	}
	text := fmt.Sprintf("%d/%d  │  ? help  │  / search  │  l level  │  q quit", pos, n)
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
	entries := m.store.Filtered()
	if len(entries) == 0 {
		msg := "Waiting for logs…"
		if m.store.Len() > 0 {
			msg = "No logs match the current filter"
		}
		return m.styles.Dim.Render(msg)
	}
	start, end := m.visibleRange(entries, m.bodyHeight())
	var b strings.Builder
	for i := start; i < end; i++ {
		b.WriteString(m.renderEntry(entries[i], i == m.selected))
		if i < end-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func (m Model) renderEntry(rec *record.Record, selected bool) string {
	ts := "—"
	if !rec.Timestamp.IsZero() {
		ts = rec.Timestamp.Local().Format("2006-01-02 15:04:05")
	}
	src := rec.Source
	if src == "" {
		src = "—"
	}
	level := m.styles.Level(rec.Level).Render(fmt.Sprintf("%s %s", levelEmoji(rec.Level), rec.Level))
	header := fmt.Sprintf("%s  │ %s │ %s", level, ts, src)
	msg := "💬 " + record.FirstLine(rec.Message)
	block := header + "\n" + m.styles.Message.Render(truncate(msg, m.width-2))
	if m.expanded[rec.ID] {
		block += "\n" + m.styles.Detail.Render(m.detailText(rec))
	}
	if selected {
		return m.styles.Selected.Width(max(0, m.width-1)).Render(block)
	}
	return lipgloss.NewStyle().PaddingLeft(2).Width(max(0, m.width)).Render(block)
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
