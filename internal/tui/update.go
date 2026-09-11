package tui

import (
	"time"

	tea "charm.land/bubbletea/v2"

	"github.com/tcontardo/prettylogs/internal/config"
	"github.com/tcontardo/prettylogs/internal/parser"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.searchInput.SetWidth(max(10, m.width-20))
		return m, nil

	case tea.ColorProfileMsg:
		m.colorProfile = msg.String()
		return m, nil

	case logMsg:
		wasFollow := m.follow
		m.store.Add(msg.rec)
		if wasFollow {
			n := len(m.rows())
			if n > 0 {
				m.selected = n - 1
			}
		}
		m.clampSelected()
		cmd := m.applyCompilePhase(msg.rec.Message)
		return m, tea.Batch(waitForLog(m.logs), cmd)

	case listenTickMsg:
		m.promoteIfListening()
		return m, listenTickCmd(m)

	case sourceDoneMsg:
		if m.quitting || m.restarting {
			return m, nil
		}
		if m.wrapper != nil {
			code, ok := m.wrapper.ExitCode()
			if !ok {
				return m, nil
			}
			m.exitCode = &code
		}
		m.status = statusExited
		return m, nil

	case quitDoneMsg:
		return m, tea.Quit

	case restartDoneMsg:
		m.restarting = false
		if m.quitting {
			return m, nil
		}
		if msg.err != nil {
			m.status = statusExited
			return m, nil
		}
		m.status = statusCompiling
		m.compileHold = false
		m.exitCode = nil
		m.follow = true
		if n := len(m.rows()); n > 0 {
			m.selected = n - 1
		}
		return m, tea.Batch(waitForDone(m.wrapper), listenTickCmd(m))

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.theming {
		return m.handleThemeKey(key)
	}
	if m.help {
		return m.handleHelpKey(key)
	}
	if m.searching {
		return m.handleSearch(msg)
	}
	if m.filtering {
		return m.handleFilter(key)
	}

	if key != "y" && key != "Y" && key != "shift+y" {
		m.copyStatus = ""
	}

	switch key {
	case "ctrl+c", "q":
		return m.quit()
	case "y":
		m.copySelection(false)
		return m, nil
	case "Y", "shift+y":
		m.copySelection(true)
		return m, nil
	case "r":
		return m, m.restartCmd()
	case "?":
		m.help = true
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g":
		m.goFirst()
	case "G", "shift+g":
		m.goLast()
	case "pgdown", "ctrl+j":
		m.movePage(1)
	case "pgup", "ctrl+k":
		m.movePage(-1)
	case "enter":
		return m, m.toggleExpand()
	case "/":
		m.searching = true
		m.searchInput.SetValue(m.store.Query())
		return m, m.searchInput.Focus()
	case "n":
		m.move(1)
	case "N", "shift+n":
		m.move(-1)
	case "l":
		m.filtering = true
		m.filterIdx = indexOf(levelOptions, m.store.Level())
		if m.filterIdx < 0 {
			m.filterIdx = 0
		}
	}
	return m, nil
}

func (m Model) handleHelpKey(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "t":
		m.theming = true
		m.themeIdx = themeIndexByName(m.themeName)
		return m, nil
	case "?", "esc":
		m.help = false
	case "q":
		m.help = false
		if !m.searching {
			return m.quit()
		}
	}
	return m, nil
}

func (m Model) handleThemeKey(key string) (tea.Model, tea.Cmd) {
	presets := themePresets()
	switch key {
	case "j", "down":
		if m.themeIdx < len(presets)-1 {
			m.themeIdx++
		}
	case "k", "up":
		if m.themeIdx > 0 {
			m.themeIdx--
		}
	case "enter":
		p := presets[m.themeIdx]
		m.styles = NewStyles(p.Theme)
		m.themeName = p.Name
		m.theming = false
		m.help = false
		if err := persistTheme(config.Config{Theme: p.Theme}); err != nil {
			m.copyStatus = "theme save failed: " + err.Error()
		}
		return m, nil
	case "esc":
		m.theming = false
		return m, nil
	case "q", "ctrl+c":
		return m.quit()
	}
	return m, nil
}

func (m *Model) goFirst() {
	if r, ok := m.currentRow(); ok && m.detailScrollActive(r) {
		m.detailScrollKey = r.key()
		m.detailScrollOffset = 0
		return
	}
	m.selected = 0
	m.follow = false
}

func (m *Model) goLast() {
	if r, ok := m.currentRow(); ok && m.detailScrollActive(r) {
		lines, visible, _, _ := m.detailWindow(r)
		m.detailScrollKey = r.key()
		m.detailScrollOffset = clamp(len(lines)-len(visible), 0, len(lines)-len(visible))
		return
	}
	if n := len(m.rows()); n > 0 {
		m.selected = n - 1
	}
	m.follow = true
}

func (m *Model) toggleExpand() tea.Cmd {
	r, ok := m.currentRow()
	if !ok {
		return nil
	}
	key := r.key()
	if m.expanded[key] {
		delete(m.expanded, key)
		if m.detailScrollKey == key {
			m.detailScrollOffset = 0
		}
	} else {
		m.expanded[key] = true
	}
	m.syncExpandFreeze()
	return listenTickCmd(*m)
}

func (m Model) handleSearch(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()
	switch key {
	case "esc":
		if m.searchInput.Value() == "" {
			m.searching = false
			m.searchInput.Blur()
			m.store.SetSearch("")
			m.clampSelected()
			return m, nil
		}
		m.searchInput.SetValue("")
		m.store.SetSearch("")
		m.clampSelected()
		return m, nil
	case "enter":
		m.store.SetSearch(m.searchInput.Value())
		m.searching = false
		m.searchInput.Blur()
		m.clampSelected()
		m.follow = false
		return m, nil
	case "ctrl+c":
		return m.quit()
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m Model) handleFilter(key string) (tea.Model, tea.Cmd) {
	switch key {
	case "esc":
		m.filtering = false
	case "j", "down":
		if m.filterIdx < len(levelOptions)-1 {
			m.filterIdx++
		}
	case "k", "up":
		if m.filterIdx > 0 {
			m.filterIdx--
		}
	case "enter":
		m.store.SetLevel(levelOptions[m.filterIdx])
		m.filtering = false
		m.clampSelected()
		m.follow = false
	case "ctrl+c", "q":
		return m.quit()
	}
	return m, nil
}

func (m Model) quit() (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, nil
	}
	m.quitting = true
	if m.wrapper == nil {
		return m, tea.Quit
	}
	m.wrapper.Abandon()
	return m, func() tea.Msg {
		_ = m.wrapper.Stop()
		return quitDoneMsg{}
	}
}

func (m *Model) restartCmd() tea.Cmd {
	if m.quitting || m.restarting || m.wrapper == nil || !m.wrapper.CanRestart() {
		return nil
	}
	m.restarting = true
	return func() tea.Msg {
		return restartDoneMsg{err: m.wrapper.Restart()}
	}
}

func (m *Model) applyCompilePhase(line string) tea.Cmd {
	if m.status == statusExited || m.quitting || m.restarting {
		return nil
	}
	start, end := parser.CompilePhase(line)
	if start {
		m.compileHold = true
		m.status = statusCompiling
		return nil
	}
	if end {
		m.compileHold = false
		m.promoteIfListening()
		return listenTickCmd(*m)
	}
	return nil
}

func (m *Model) promoteIfListening() {
	if m.status == statusExited || m.compileHold || m.wrapper == nil {
		return
	}
	if m.wrapper.HasListeningPort() {
		m.status = statusRunning
	}
}

func (m *Model) syncExpandFreeze() {
	if !m.hasExpanded() {
		m.freezeID = 0
		m.frozenCounts = nil
		return
	}
	if m.freezeID == 0 {
		m.freezeID = m.store.MaxID()
		m.frozenCounts = m.store.Counts()
	}
}

func (m Model) hasExpanded() bool {
	return len(m.expanded) > 0
}

func needsListenTick(m Model) bool {
	if m.wrapper == nil || m.quitting || m.restarting {
		return false
	}
	if m.status == statusExited || m.status == statusRunning {
		return false
	}
	if m.compileHold || m.hasExpanded() {
		return false
	}
	return true
}

func listenTickCmd(m Model) tea.Cmd {
	if !needsListenTick(m) {
		return nil
	}
	return tea.Tick(500*time.Millisecond, func(time.Time) tea.Msg {
		return listenTickMsg{}
	})
}

func indexOf(items []string, want string) int {
	for i, v := range items {
		if v == want {
			return i
		}
	}
	return -1
}

func (m Model) entryHeight(r row) int {
	if !m.expanded[r.key()] {
		if r.kind == rowGroup {
			return 2 // header + "folded" hint — unchanged collapsed group shape
		}
		return 1 // dense single-line record
	}
	_, visible, indicator, _ := m.detailWindow(r)
	h := 1 + len(visible)
	if indicator {
		h++
	}
	return h
}

func (m Model) visibleRange(rows []row, bodyHeight int) (int, int) {
	if len(rows) == 0 || bodyHeight <= 0 {
		return 0, 0
	}
	sel := m.selected
	if sel < 0 {
		sel = 0
	}
	if sel >= len(rows) {
		sel = len(rows) - 1
	}
	start := sel
	used := m.entryHeight(rows[sel])
	for start > 0 && used+m.entryHeight(rows[start-1]) <= bodyHeight {
		start--
		used += m.entryHeight(rows[start])
	}
	end := sel + 1
	for end < len(rows) && used+m.entryHeight(rows[end]) <= bodyHeight {
		used += m.entryHeight(rows[end])
		end++
	}
	for start > 0 && used+m.entryHeight(rows[start-1]) <= bodyHeight {
		start--
		used += m.entryHeight(rows[start])
	}
	return start, end
}
