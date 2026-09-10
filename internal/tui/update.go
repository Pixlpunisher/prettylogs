package tui

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	tea "charm.land/bubbletea/v2"
)

var debugConsumeCount int

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
		// #region agent log
		debugConsumeCount++
		if debugConsumeCount == 1 || debugConsumeCount%25 == 0 {
			preview := msg.rec.Message
			if len(preview) > 80 {
				preview = preview[:80]
			}
			tuiAgentLog("A", "update.go:logMsg", "tui consumed", map[string]any{"n": debugConsumeCount, "storeLen": m.store.Len(), "preview": preview})
		}
		// #endregion
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
		// #region agent log
		tuiAgentLog("D", "update.go:sourceDoneMsg", "source done", map[string]any{"status": m.sourceStatus, "consumed": debugConsumeCount, "storeLen": m.store.Len()})
		// #endregion
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
		if r, ok := m.currentRow(); ok && m.detailScrollActive(r) {
			m.detailScrollKey = r.key()
			m.detailScrollOffset = 0
		} else {
			m.selected = 0
			m.follow = false
		}
	case "G", "shift+g":
		if r, ok := m.currentRow(); ok && m.detailScrollActive(r) {
			lines, visible, _, _ := m.detailWindow(r)
			max := len(lines) - len(visible)
			if max < 0 {
				max = 0
			}
			m.detailScrollKey = r.key()
			m.detailScrollOffset = max
		} else {
			n := len(m.rows())
			if n > 0 {
				m.selected = n - 1
			}
			m.follow = true
		}
	case "pgdown", "ctrl+j":
		m.movePage(1)
	case "pgup", "ctrl+k":
		m.movePage(-1)
	case "enter":
		if r, ok := m.currentRow(); ok {
			key := r.key()
			m.expanded[key] = !m.expanded[key]
			if !m.expanded[key] && m.detailScrollKey == key {
				m.detailScrollOffset = 0
			}
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

// #region agent log
func tuiAgentLog(hid, loc, msg string, data map[string]any) {
	f, err := os.OpenFile("/Users/tcontardo/Github/PrettyLogs/.cursor/debug-c9d99c.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(map[string]any{
		"sessionId":    "c9d99c",
		"hypothesisId": hid,
		"location":     loc,
		"message":      msg,
		"data":         data,
		"timestamp":    time.Now().UnixMilli(),
	})
}

// #endregion
