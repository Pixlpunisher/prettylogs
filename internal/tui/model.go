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

type listenTickMsg struct{}

type procStatus string

const (
	statusCompiling procStatus = "compiling"
	statusRunning   procStatus = "running"
	statusExited    procStatus = "exited"
)

type Model struct {
	store   *store.Store
	logs    <-chan record.Record
	wrapper *wrapper.Wrapper
	styles  Styles

	width    int
	height   int
	selected int
	expanded map[uint64]bool

	detailScrollKey    uint64 // row.key() the offset below applies to; 0 = none
	detailScrollOffset int    // first visible detail-content line index

	searchInput textinput.Model
	searching   bool
	filtering   bool
	filterIdx   int
	help        bool

	status      procStatus
	exitCode    *int
	compileHold bool // log-detected compile; listen port must not override
	follow      bool

	colorProfile string // diagnostic: what bubbletea detected, shown in help

	freezeID     uint64         // last record ID visible while a row is expanded; 0 = live
	frozenCounts map[string]int // header counts pinned at expand
}

func New(st *store.Store, logs <-chan record.Record, cfg config.Config, w *wrapper.Wrapper) Model {
	ti := textinput.New()
	ti.Prompt = "/ "
	ti.Placeholder = "regex"
	ti.SetWidth(40)
	return Model{
		store:       st,
		logs:        logs,
		wrapper:     w,
		styles:      NewStyles(cfg.Theme),
		expanded:    make(map[uint64]bool),
		searchInput: ti,
		status:      initialStatus(w),
		follow:      true,
		width:       80,
		height:      24,
	}
}

func initialStatus(w *wrapper.Wrapper) procStatus {
	if w != nil {
		return statusCompiling
	}
	return statusRunning
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForLog(m.logs), listenTickCmd(m))
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
	n := len(m.rows())
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
	if r, ok := m.currentRow(); ok && m.detailScrollActive(r) {
		m.scrollDetail(r, delta)
		return
	}
	n := len(m.rows())
	if n == 0 {
		m.selected = 0
		m.follow = true
		return
	}
	m.selected += delta
	m.clampSelected()
	m.follow = m.selected == n-1
}

// movePage pages in direction dir (-1/+1): a page of rows via pageSize()
// when navigating the row list, or a page of detail-content lines
// (detailCapacity()) when the selected row is in detail-scroll mode — these
// are different units, so paging is split out of move() rather than having
// move() reinterpret its delta by magnitude.
func (m *Model) movePage(dir int) {
	if r, ok := m.currentRow(); ok && m.detailScrollActive(r) {
		m.scrollDetail(r, dir*m.detailCapacity())
		return
	}
	m.move(dir * m.pageSize())
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

func (m Model) currentRow() (row, bool) {
	rows := m.rows()
	if m.selected < 0 || m.selected >= len(rows) {
		return row{}, false
	}
	return rows[m.selected], true
}
