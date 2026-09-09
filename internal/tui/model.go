package tui

import (
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"

	"github.com/tcontardo/prettylogs/internal/config"
	"github.com/tcontardo/prettylogs/internal/record"
	"github.com/tcontardo/prettylogs/internal/store"
	"github.com/tcontardo/prettylogs/internal/wrapper"
)

type logMsg struct {
	rec record.Record
}

type sourceDoneMsg struct{}

type Model struct {
	store   *store.Store
	logs    <-chan record.Record
	wrapper *wrapper.Wrapper
	styles  Styles

	width    int
	height   int
	selected int
	expanded map[uint64]bool

	searchInput textinput.Model
	searching   bool
	filtering   bool
	filterIdx   int
	help        bool

	sourceStatus string
	follow       bool
}

func New(st *store.Store, logs <-chan record.Record, cfg config.Config, w *wrapper.Wrapper, source string) Model {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "regex"
	ti.SetWidth(40)
	status := source
	if status == "" {
		status = "running"
	}
	return Model{
		store:        st,
		logs:         logs,
		wrapper:      w,
		styles:       NewStyles(cfg.Theme),
		expanded:     make(map[uint64]bool),
		searchInput:  ti,
		sourceStatus: status,
		follow:       true,
		width:        80,
		height:       24,
	}
}

func (m Model) Init() tea.Cmd {
	return waitForLog(m.logs)
}

func waitForLog(ch <-chan record.Record) tea.Cmd {
	if ch == nil {
		return nil
	}
	return func() tea.Msg {
		rec, ok := <-ch
		if !ok {
			return sourceDoneMsg{}
		}
		return logMsg{rec: rec}
	}
}

func (m *Model) clampSelected() {
	n := len(m.store.Filtered())
	if n == 0 {
		m.selected = 0
		return
	}
	if m.selected >= n {
		m.selected = n - 1
	}
	if m.selected < 0 {
		m.selected = 0
	}
}

func (m *Model) move(delta int) {
	n := len(m.store.Filtered())
	if n == 0 {
		m.selected = 0
		m.follow = true
		return
	}
	m.selected += delta
	m.clampSelected()
	m.follow = m.selected == n-1
}

func (m *Model) pageSize() int {
	h := m.bodyHeight()
	if h < 3 {
		return 1
	}
	return max(1, h/3)
}

func (m *Model) bodyHeight() int {
	h := m.height - 3
	if m.searching {
		h--
	}
	if m.filtering {
		h -= 7
	}
	if h < 1 {
		return 1
	}
	return h
}

func (m Model) currentRecord() *record.Record {
	entries := m.store.Filtered()
	if m.selected < 0 || m.selected >= len(entries) {
		return nil
	}
	return entries[m.selected]
}
