package tui

import (
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"

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
