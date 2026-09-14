package tui

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tcontardo/prettylogs/internal/record"
)

// wordmark is the one-line header/help title (wood emoji + unicode small caps).
const wordmark = "🪵 ᴘʀᴇᴛᴛʏʟᴏɢꜱ"

func (m Model) View() tea.View {
	body := m.mainView()
	// Help and the theme picker are modals: the log list stays on screen
	// behind them rather than being replaced by a separate page.
	if m.theming {
		body = overlayCentered(body, m.themeView(), m.width, m.height)
	} else if m.help {
		body = overlayCentered(body, m.helpView(), m.width, m.height)
	}
	v := tea.NewView(body)
	v.AltScreen = true
	return v
}

// overlayCentered draws box on top of base, centered, leaving the rest of
// base visible. Lines are spliced by display width so ANSI styling in either
// layer is preserved.
func overlayCentered(base, box string, width, height int) string {
	if width <= 0 || height <= 0 {
		return box
	}
	baseLines := strings.Split(base, "\n")
	boxLines := strings.Split(box, "\n")
	boxW, boxH := lipgloss.Width(box), len(boxLines)
	top := max(0, (height-boxH)/2)
	left := max(0, (width-boxW)/2)

	for i, boxLine := range boxLines {
		y := top + i
		if y >= len(baseLines) {
			break
		}
		baseLines[y] = spliceLine(baseLines[y], boxLine, left, width)
	}
	return strings.Join(baseLines, "\n")
}

// spliceLine replaces the run of cells [at, at+width(overlay)) in base with
// overlay, padding base with spaces when it is shorter than the insert point.
func spliceLine(base, overlay string, at, width int) string {
	prefix := ansi.Truncate(base, at, "")
	if pad := at - lipgloss.Width(prefix); pad > 0 {
		prefix += strings.Repeat(" ", pad)
	}
	tailStart := at + lipgloss.Width(overlay)
	suffix := ""
	if lipgloss.Width(base) > tailStart {
		suffix = ansi.TruncateLeft(base, tailStart, "")
	}
	line := prefix + overlay + suffix
	if lipgloss.Width(line) > width {
		line = ansi.Truncate(line, width, "")
	}
	return line
}

// mainView stacks header, list and footer, padding between the list and the
// footer so the footer always lands on the terminal's last line instead of
// floating directly under a short list.
func (m Model) mainView() string {
	// Computed once and shared: listView and footerView both need the same
	// rows, and rows() does a store copy plus a grouping pass over it.
	rows := m.rows()

	var top strings.Builder
	top.WriteString(m.headerView())
	top.WriteString("\n")
	if m.filtering {
		top.WriteString(m.filterView())
		top.WriteString("\n")
	}
	top.WriteString(m.listView(rows))

	var bottom strings.Builder
	if m.searching {
		bottom.WriteString(m.searchInput.View())
		if err := m.store.QueryError(); err != nil {
			bottom.WriteString("  ")
			bottom.WriteString(m.styles.SearchErr.Render(err.Error()))
		}
		bottom.WriteString("\n")
	}
	bottom.WriteString(m.footerView(rows))

	// The separator newlines themselves start new lines, so joining a topH-line
	// and a bottomH-line block with gap newlines yields topH+gap+bottomH-1 lines.
	gap := m.height - lipgloss.Height(top.String()) - lipgloss.Height(bottom.String()) + 1
	return top.String() + strings.Repeat("\n", max(1, gap)) + bottom.String()
}

func (m Model) headerView() string {
	counts := m.store.Counts()
	if m.frozenCounts != nil {
		counts = m.frozenCounts
	}
	filter := m.store.Query()
	if filter == "" {
		filter = "_______"
	}
	level := m.store.Level()
	title := m.styles.Wordmark.Render(wordmark)
	search := fmt.Sprintf("🔍 Filter: [%s]", filter)
	stats := fmt.Sprintf("📊 ERROR %d │ WARN %d │ INFO %d │ DEBUG %d",
		counts[record.LevelError], counts[record.LevelWarn], counts[record.LevelInfo], counts[record.LevelDebug])
	badge := m.statusBadge()
	meta := m.styles.Dim.Render(fmt.Sprintf("%s │ %s", search, stats)) + " │ " + badge
	if level != "All" {
		meta = m.styles.Dim.Render(fmt.Sprintf("%s │ level:%s", search, level)) + " │ " + badge
	}
	line := title + " │ " + meta
	if m.width > 0 {
		return lipgloss.NewStyle().MaxWidth(m.width).Render(line)
	}
	return line
}

func (m Model) statusBadge() string {
	if m.quitting {
		return m.styles.StatusExited.Render("● Shutting down..")
	}
	switch m.status {
	case statusCompiling:
		return m.styles.StatusCompiling.Render("● Compiling..")
	case statusExited:
		label := "● Exited"
		if m.exitCode != nil {
			label = fmt.Sprintf("● Exited %d", *m.exitCode)
		}
		return m.styles.StatusExited.Render(label)
	default:
		return m.styles.StatusRunning.Render("● Running")
	}
}

func (m Model) footerView(rows []row) string {
	n := len(rows)
	pos := 0
	if n > 0 {
		pos = m.selected + 1
	}
	text := fmt.Sprintf("%d/%d  │  ? help  │  / search  │  y pretty copy  │  Y raw copy  │  l level  │  r restart │  q quit", pos, n)
	if m.copyStatus != "" {
		text = m.copyStatus + "  │  " + text
	}
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

func (m Model) listView(rows []row) string {
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
	body, extrasOK := detailBody(rec)
	if !extrasOK {
		return body
	}
	extras := formatExtra(rec.Extra)
	if body != "" && extras != "" {
		return body + "\n" + extras
	}
	if body != "" {
		return body
	}
	return extras
}

func detailBody(rec *record.Record) (body string, extrasOK bool) {
	first := record.FirstLine(rec.Raw)
	rest := strings.TrimPrefix(rec.Raw, first)
	rest = strings.TrimPrefix(rest, "\n")
	if rest != "" {
		return rest, true
	}
	if rec.Message != record.FirstLine(rec.Message) {
		return strings.TrimPrefix(rec.Message, record.FirstLine(rec.Message)+"\n"), true
	}
	if pretty := prettyJSON(rec.Raw); pretty != "" && pretty != record.FirstLine(rec.Message) {
		return pretty, false
	}
	return "", true
}

func formatExtra(extra map[string]string) string {
	if len(extra) == 0 {
		return ""
	}
	keys := make([]string, 0, len(extra))
	for k := range extra {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for i, k := range keys {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(fmt.Sprintf("%s: %s", k, extra[k]))
	}
	return b.String()
}

func prettyJSON(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if !json.Valid([]byte(trimmed)) {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(trimmed), "", "  "); err != nil {
		return ""
	}
	return buf.String()
}

// helpChrome is the horizontal space the modal's border (1 each side) and
// Help padding (2 each side) take from Width().
const helpChrome = 6

// modalWidth is the total width of the help/theme boxes: as wide as the
// terminal allows up to 4/5 of it, leaving a gutter so the list shows through.
func (m Model) modalWidth() int {
	return max(56, min(m.width-4, m.width*4/5))
}

// helpCapacity is how many help lines fit in the modal, leaving room for the
// box border and the surrounding header/footer.
func (m Model) helpCapacity() int {
	return max(6, m.height-6)
}

// helpWindow returns the full help lines, the slice visible at the clamped
// scroll offset, whether a scroll indicator is needed, and that offset.
func (m Model) helpWindow() (lines, visible []string, indicator bool, offset int) {
	// Wrap to the box's inner width first so one element of lines is always
	// one rendered row; otherwise a wrapped line would make the modal taller
	// than the window we budgeted for.
	lines = wrapToWidth(strings.Split(formatHelp(m.styles), "\n"), m.modalWidth()-helpChrome)
	capacity := m.helpCapacity()
	if len(lines) <= capacity {
		return lines, lines, false, 0
	}
	visibleN := max(1, capacity-1) // last line shows the scroll indicator
	offset = clamp(m.helpScroll, 0, len(lines)-visibleN)
	return lines, lines[offset : offset+visibleN], true, offset
}

func (m Model) maxHelpScroll() int {
	lines, visible, indicator, _ := m.helpWindow()
	if !indicator {
		return 0
	}
	return max(0, len(lines)-len(visible))
}

func (m Model) helpView() string {
	lines, visible, indicator, offset := m.helpWindow()
	body := strings.Join(visible, "\n")
	if indicator {
		label := fmt.Sprintf("── lines %d-%d of %d · j/k scroll ──",
			offset+1, offset+len(visible), len(lines))
		body += "\n" + m.styles.Dim.Render(label)
	}
	return m.styles.Help.
		Border(lipgloss.NormalBorder()).
		BorderForeground(themeColor("dim")).
		Width(m.modalWidth()).
		Render(body)
}

func (m Model) themeView() string {
	presets := themePresets()
	var b strings.Builder
	b.WriteString(m.styles.Header.Render("Choose theme"))
	b.WriteString("\n")
	for i, p := range presets {
		cursor := "  "
		if i == m.themeIdx {
			cursor = "> "
		}
		line := cursor + p.Name
		if i == m.themeIdx {
			line = m.styles.Selected.Render(line)
		}
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(themePreviewLine(presets[m.themeIdx].Theme))
	b.WriteString("\n")
	b.WriteString(m.styles.Dim.Render("Enter to apply, Esc to cancel"))
	box := m.styles.Help.
		Border(lipgloss.NormalBorder()).
		BorderForeground(themeColor("dim")).
		Width(m.modalWidth()).
		Render(strings.TrimRight(b.String(), "\n"))
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
