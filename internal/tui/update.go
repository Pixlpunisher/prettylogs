package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"

	"github.com/tcontardo/prettylogs/internal/record"
)

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.searchInput.SetWidth(max(10, m.width-20))
		return m, nil

	case logMsg:
		wasFollow := m.follow
		m.store.Add(msg.rec)
		if wasFollow {
			n := len(m.store.Filtered())
			if n > 0 {
				m.selected = n - 1
			}
		}
		m.clampSelected()
		return m, waitForLog(m.logs)

	case sourceDoneMsg:
		if m.wrapper != nil {
			if code, ok := m.wrapper.ExitCode(); ok {
				m.sourceStatus = fmt.Sprintf("exited %d", code)
			} else {
				m.sourceStatus = "exited"
			}
		} else if m.sourceStatus == "running" || m.sourceStatus == "stdin" {
			m.sourceStatus = "eof"
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	key := msg.String()

	if m.help {
		if key == "?" || key == "esc" || key == "q" {
			m.help = false
			if key == "q" && !m.searching {
				return m.quit()
			}
		}
		return m, nil
	}

	if m.searching {
		return m.handleSearch(msg)
	}

	if m.filtering {
		return m.handleFilter(key)
	}

	switch key {
	case "ctrl+c", "q":
		return m.quit()
	case "?":
		m.help = true
	case "j", "down":
		m.move(1)
	case "k", "up":
		m.move(-1)
	case "g":
		m.selected = 0
		m.follow = false
	case "G", "shift+g":
		n := len(m.store.Filtered())
		if n > 0 {
			m.selected = n - 1
		}
		m.follow = true
	case "pgdown", "ctrl+j":
		m.move(m.pageSize())
	case "pgup", "ctrl+k":
		m.move(-m.pageSize())
	case "enter":
		if rec := m.currentRecord(); rec != nil {
			m.expanded[rec.ID] = !m.expanded[rec.ID]
		}
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
	if m.wrapper != nil {
		_ = m.wrapper.Stop()
	}
	return m, tea.Quit
}

func indexOf(items []string, want string) int {
	for i, v := range items {
		if v == want {
			return i
		}
	}
	return -1
}

func (m Model) entryHeight(rec *record.Record) int {
	if rec == nil {
		return 0
	}
	h := 2
	if m.expanded[rec.ID] {
		extra := 0
		raw := rec.Raw
		for i := 0; i < len(raw); i++ {
			if raw[i] == '\n' {
				extra++
			}
		}
		if extra == 0 && rec.Message != record.FirstLine(rec.Message) {
			for i := 0; i < len(rec.Message); i++ {
				if rec.Message[i] == '\n' {
					extra++
				}
			}
		}
		extra += len(rec.Extra)
		if extra < 1 {
			extra = 1
		}
		h += extra
	}
	return h
}

func (m Model) visibleRange(entries []*record.Record, bodyHeight int) (int, int) {
	if len(entries) == 0 || bodyHeight <= 0 {
		return 0, 0
	}
	sel := m.selected
	if sel < 0 {
		sel = 0
	}
	if sel >= len(entries) {
		sel = len(entries) - 1
	}
	start := sel
	used := m.entryHeight(entries[sel])
	for start > 0 && used+m.entryHeight(entries[start-1]) <= bodyHeight {
		start--
		used += m.entryHeight(entries[start])
	}
	end := sel + 1
	for end < len(entries) && used+m.entryHeight(entries[end]) <= bodyHeight {
		used += m.entryHeight(entries[end])
		end++
	}
	for start > 0 && used+m.entryHeight(entries[start-1]) <= bodyHeight {
		start--
		used += m.entryHeight(entries[start])
	}
	return start, end
}
