package tui

import (
	"strconv"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"

	"github.com/tcontardo/prettylogs/internal/config"
	"github.com/tcontardo/prettylogs/internal/record"
	"github.com/tcontardo/prettylogs/internal/store"
)

func keyMsg(s string) tea.KeyPressMsg {
	return tea.KeyPressMsg{Text: s}
}

func newTestModel(recs ...record.Record) Model {
	st := store.New(100)
	for _, rec := range recs {
		st.Add(rec)
	}
	m := New(st, nil, config.Default(), nil, "stdin")
	m.width = 80
	m.height = 24
	return m
}

func TestViewRendersCollapsedBlock(t *testing.T) {
	t.Parallel()
	m := newTestModel(record.Record{
		Level:     record.LevelError,
		Message:   "Database connection failed",
		Source:    "auth.service.ts:124",
		Timestamp: time.Date(2026, 9, 9, 10, 23, 45, 0, time.UTC),
		Raw:       "Database connection failed",
	})
	content := m.View().Content
	for _, want := range []string{"prettylogs", "ERROR", "Database connection failed", "auth.service.ts:124"} {
		if !strings.Contains(content, want) {
			t.Fatalf("view missing %q:\n%s", want, content)
		}
	}
}

func TestEnterTogglesDetail(t *testing.T) {
	t.Parallel()
	m := newTestModel(record.Record{
		Level:   record.LevelError,
		Message: "boom",
		Raw:     "boom\n  at Pool.connect (/src/db/pool.ts:45)",
	})
	collapsed := m.View().Content
	if strings.Contains(collapsed, "Pool.connect") {
		t.Fatal("stack should be hidden when collapsed")
	}
	updated, _ := m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	expanded := m.View().Content
	if !strings.Contains(expanded, "Pool.connect") {
		t.Fatalf("stack should show when expanded:\n%s", expanded)
	}
}

func TestLevelFilterHidesOtherLevels(t *testing.T) {
	t.Parallel()
	m := newTestModel(
		record.Record{Level: record.LevelInfo, Message: "listening", Raw: "listening"},
		record.Record{Level: record.LevelError, Message: "boom", Raw: "boom"},
	)
	m.store.SetLevel(record.LevelError)
	m.clampSelected()
	content := m.View().Content
	if strings.Contains(content, "listening") {
		t.Fatalf("info should be filtered out:\n%s", content)
	}
	if !strings.Contains(content, "boom") {
		t.Fatalf("error should remain:\n%s", content)
	}
}

func infoRecords(n int, prefix string) []record.Record {
	out := make([]record.Record, n)
	for i := range out {
		msg := prefix + string(rune('0'+i))
		out[i] = record.Record{Level: record.LevelInfo, Message: msg, Raw: msg}
	}
	return out
}

func TestViewLeavesShortRunUncollapsed(t *testing.T) {
	t.Parallel()
	recs := infoRecords(3, "info-")
	recs = append(recs, record.Record{Level: record.LevelError, Message: "boom", Raw: "boom"})
	m := newTestModel(recs...)
	content := m.View().Content
	for _, want := range []string{"info-0", "info-1", "info-2", "boom"} {
		if !strings.Contains(content, want) {
			t.Fatalf("view missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "folded") {
		t.Fatalf("short run should not collapse:\n%s", content)
	}
}

func TestViewCollapsesRunAtThreshold(t *testing.T) {
	t.Parallel()
	recs := infoRecords(4, "info-")
	recs = append(recs, record.Record{Level: record.LevelError, Message: "boom", Raw: "boom"})
	m := newTestModel(recs...)
	content := m.View().Content
	if !strings.Contains(content, "×4") || !strings.Contains(content, "folded") {
		t.Fatalf("expected collapsed group summary:\n%s", content)
	}
	for _, hidden := range []string{"info-0", "info-1", "info-2", "info-3"} {
		if strings.Contains(content, hidden) {
			t.Fatalf("collapsed group should hide %q:\n%s", hidden, content)
		}
	}
	if !strings.Contains(content, "boom") {
		t.Fatalf("error row should still render normally:\n%s", content)
	}
}

func TestEnterExpandsGroupShowsAllMembers(t *testing.T) {
	t.Parallel()
	m := newTestModel(infoRecords(5, "info-")...)
	collapsed := m.View().Content
	for i := 0; i < 5; i++ {
		if strings.Contains(collapsed, "info-"+string(rune('0'+i))) {
			t.Fatalf("collapsed view should hide info-%d:\n%s", i, collapsed)
		}
	}
	updated, _ := m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	expanded := m.View().Content
	for i := 0; i < 5; i++ {
		want := "info-" + string(rune('0'+i))
		if !strings.Contains(expanded, want) {
			t.Fatalf("expanded view missing %q:\n%s", want, expanded)
		}
	}
}

func TestEnterCollapsesGroupBack(t *testing.T) {
	t.Parallel()
	m := newTestModel(infoRecords(5, "info-")...)
	updated, _ := m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	content := m.View().Content
	if !strings.Contains(content, "folded") {
		t.Fatalf("expected group to collapse back:\n%s", content)
	}
	if strings.Contains(content, "info-0") {
		t.Fatalf("collapsed group should hide members again:\n%s", content)
	}
}

func TestFooterCountsRowsNotRecords(t *testing.T) {
	t.Parallel()
	recs := infoRecords(5, "info-")
	recs = append(recs, record.Record{Level: record.LevelError, Message: "boom", Raw: "boom"})
	m := newTestModel(recs...)
	content := m.View().Content
	if !strings.Contains(content, "1/2") {
		t.Fatalf("footer should count 2 rows (1 group + 1 record), not 6 records:\n%s", content)
	}
}

func TestVisibleRangeAccountsForGroupHeight(t *testing.T) {
	t.Parallel()
	m := newTestModel(infoRecords(5, "info-")...)
	rows := m.rows()
	if len(rows) != 1 {
		t.Fatalf("expected 1 group row, got %d", len(rows))
	}
	start, end := m.visibleRange(rows, 2)
	if start != 0 || end != 1 {
		t.Fatalf("collapsed group height 2 should fit budget 2: got [%d,%d)", start, end)
	}
	m.expanded[rows[0].key()] = true
	rows = m.rows()
	start, end = m.visibleRange(rows, 2)
	if start != 0 || end != 1 {
		t.Fatalf("expanded group still only row available: got [%d,%d)", start, end)
	}
	if h := m.entryHeight(rows[0]); h != 1+5 {
		t.Fatalf("expanded group height: got %d want %d", h, 1+5)
	}
}

func TestRenderEntryIsSingleDenseLine(t *testing.T) {
	t.Parallel()
	ts := time.Date(2026, 9, 9, 10, 23, 45, 0, time.UTC)
	m := newTestModel(record.Record{
		Level:     record.LevelError,
		Message:   "Database connection failed",
		Source:    "auth.service.ts:124",
		Timestamp: ts,
		Raw:       "Database connection failed",
	})
	content := m.View().Content
	for _, want := range []string{"[ERR]", ts.Local().Format("15:04:05"), "auth.service.ts:124", "Database connection failed"} {
		if !strings.Contains(content, want) {
			t.Fatalf("view missing %q:\n%s", want, content)
		}
	}
	if strings.Contains(content, "💬") {
		t.Fatalf("chat emoji should be removed from the dense row:\n%s", content)
	}
	if strings.Contains(content, "2026-09-09") {
		t.Fatalf("date should be dropped in favor of time-only display:\n%s", content)
	}
}

func TestRenderEntryEmptySourceShowsDash(t *testing.T) {
	t.Parallel()
	m := newTestModel(record.Record{
		Level:   record.LevelInfo,
		Message: "listening on :8080",
		Raw:     "listening on :8080",
	})
	content := m.View().Content
	if !strings.Contains(content, "│ — │") {
		t.Fatalf("expected em-dash placeholder for empty source, not a misaligned gap:\n%s", content)
	}
}

func TestSpineUsesThickBorderForSelectedRow(t *testing.T) {
	t.Parallel()
	m := newTestModel(
		record.Record{Level: record.LevelError, Message: "boom", Raw: "boom"},
		record.Record{Level: record.LevelInfo, Message: "listening", Raw: "listening"},
	)
	m.selected = 0
	content := m.View().Content
	lines := strings.Split(content, "\n")
	var selectedLine, otherLine string
	for _, l := range lines {
		if strings.Contains(l, "boom") {
			selectedLine = l
		}
		if strings.Contains(l, "listening") {
			otherLine = l
		}
	}
	if !strings.HasPrefix(ansi.Strip(selectedLine), "┃") {
		t.Fatalf("selected row should use the thick spine border, got: %q", selectedLine)
	}
	if !strings.HasPrefix(ansi.Strip(otherLine), "│") {
		t.Fatalf("non-selected row should use the normal spine border, got: %q", otherLine)
	}
}

func TestSearchHighlightsMatchInMessage(t *testing.T) {
	t.Parallel()
	m := newTestModel(
		record.Record{Level: record.LevelInfo, Message: "Redis reconnection attempt", Raw: "Redis reconnection attempt"},
	)
	m.store.SetSearch("Redis")
	content := m.View().Content
	if !strings.Contains(content, "reconnection attempt") {
		t.Fatalf("non-matched part of the message should still be visible:\n%s", content)
	}
	// The highlight style (Reverse+Bold) wraps just the matched span with
	// its own SGR codes, distinct from the surrounding plain message style.
	if !strings.Contains(content, "\x1b[1;7mRedis\x1b[m") && !strings.Contains(content, "\x1b[7;1mRedis\x1b[m") {
		t.Fatalf("expected \"Redis\" to be wrapped in reverse+bold highlight codes:\n%q", content)
	}
}

func rawWithLines(first string, n int) string {
	lines := make([]string, n+1)
	lines[0] = first
	for i := 0; i < n; i++ {
		lines[i+1] = "L" + strconv.Itoa(i)
	}
	return strings.Join(lines, "\n")
}

func manyInfoRecords(n int, prefix string) []record.Record {
	out := make([]record.Record, n)
	for i := range out {
		msg := prefix + strconv.Itoa(i)
		out[i] = record.Record{Level: record.LevelInfo, Message: msg, Raw: msg}
	}
	return out
}

func TestExpandedRecordDetailShowsScrollIndicatorAndTop(t *testing.T) {
	t.Parallel()
	m := newTestModel(record.Record{Level: record.LevelError, Message: "boom", Raw: rawWithLines("boom", 39)})
	updated, _ := m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	content := m.View().Content
	if !strings.Contains(content, "lines 1-19 of 39") {
		t.Fatalf("expected scroll indicator at top:\n%s", content)
	}
	if !strings.Contains(content, "L0") || strings.Contains(content, "L19") {
		t.Fatalf("expected only the first window of lines visible:\n%s", content)
	}
}

func TestDetailScrollDownReachesBottomAndClamps(t *testing.T) {
	t.Parallel()
	m := newTestModel(record.Record{Level: record.LevelError, Message: "boom", Raw: rawWithLines("boom", 39)})
	updated, _ := m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("pgdown"))
	m = updated.(Model)
	content := m.View().Content
	if !strings.Contains(content, "lines 21-39 of 39") {
		t.Fatalf("expected scroll to reach the bottom after one page:\n%s", content)
	}
	if strings.Contains(content, "L0") || !strings.Contains(content, "L38") {
		t.Fatalf("expected only the bottom window of lines visible:\n%s", content)
	}
	updated, _ = m.handleKey(keyMsg("pgdown"))
	m2 := updated.(Model)
	if m2.View().Content != content {
		t.Fatal("further downward paging past the bottom should be a no-op")
	}
}

func TestDetailScrollUpMovesWindowBack(t *testing.T) {
	t.Parallel()
	m := newTestModel(record.Record{Level: record.LevelError, Message: "boom", Raw: rawWithLines("boom", 39)})
	updated, _ := m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("pgdown"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("k"))
	m = updated.(Model)
	content := m.View().Content
	if !strings.Contains(content, "lines 20-38 of 39") {
		t.Fatalf("expected scroll to move up by one line:\n%s", content)
	}
}

func TestCollapsingResetsDetailScrollForNextExpand(t *testing.T) {
	t.Parallel()
	m := newTestModel(record.Record{Level: record.LevelError, Message: "boom", Raw: rawWithLines("boom", 39)})
	updated, _ := m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("pgdown"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	collapsed := m.View().Content
	if strings.Contains(collapsed, "lines ") {
		t.Fatalf("collapsed row should show no scroll indicator:\n%s", collapsed)
	}
	updated, _ = m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	expanded := m.View().Content
	if !strings.Contains(expanded, "lines 1-19 of 39") {
		t.Fatalf("re-expanding should reset scroll to the top:\n%s", expanded)
	}
}

func TestExpandedDetailThatFitsShowsNoIndicatorAndNeighborsRemain(t *testing.T) {
	t.Parallel()
	m := newTestModel(
		record.Record{Level: record.LevelError, Message: "first", Raw: "first\nextra-detail-line"},
		record.Record{Level: record.LevelError, Message: "second boom", Raw: "second boom"},
	)
	m.selected = 0
	updated, _ := m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	content := m.View().Content
	if strings.Contains(content, "lines ") {
		t.Fatalf("short detail should not show a scroll indicator:\n%s", content)
	}
	if !strings.Contains(content, "extra-detail-line") {
		t.Fatalf("expanded detail should be visible:\n%s", content)
	}
	if !strings.Contains(content, "second boom") {
		t.Fatalf("neighboring row should still render alongside the expanded row:\n%s", content)
	}
}

func TestExpandedGroupDetailScrolls(t *testing.T) {
	t.Parallel()
	recs := manyInfoRecords(30, "info-")
	recs = append(recs, record.Record{Level: record.LevelError, Message: "boom", Raw: "boom"})
	m := newTestModel(recs...)
	m.selected = 0
	updated, _ := m.handleKey(keyMsg("enter"))
	m = updated.(Model)
	content := m.View().Content
	if !strings.Contains(content, "lines 1-19 of 30") {
		t.Fatalf("expected group detail scroll indicator:\n%s", content)
	}
	if !strings.Contains(content, "info-0") || strings.Contains(content, "info-29") {
		t.Fatalf("expected only the first window of group members visible:\n%s", content)
	}
	updated, _ = m.handleKey(keyMsg("pgdown"))
	m = updated.(Model)
	content = m.View().Content
	if !strings.Contains(content, "info-29") {
		t.Fatalf("expected to reach the last group member after paging down:\n%s", content)
	}
}

func TestHelpOverlay(t *testing.T) {
	t.Parallel()
	m := newTestModel()
	updated, _ := m.handleKey(keyMsg("?"))
	m = updated.(Model)
	content := m.View().Content
	if !strings.Contains(content, "Keyboard Shortcuts") {
		t.Fatalf("help missing:\n%s", content)
	}
}
