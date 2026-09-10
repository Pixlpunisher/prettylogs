package tui

import (
	"testing"

	"github.com/tcontardo/prettylogs/internal/record"
)

func recs(n int, level string) []*record.Record {
	out := make([]*record.Record, n)
	for i := range out {
		out[i] = &record.Record{ID: uint64(i + 1), Level: level, Message: "msg"}
	}
	return out
}

func TestBuildRowsBelowThresholdStaysIndividual(t *testing.T) {
	t.Parallel()
	rows := buildRows(recs(3, record.LevelInfo), groupThreshold)
	if len(rows) != 3 {
		t.Fatalf("rows: got %d want 3", len(rows))
	}
	for _, r := range rows {
		if r.kind != rowRecord {
			t.Fatalf("expected all rowRecord, got %+v", r)
		}
	}
}

func TestBuildRowsAtThresholdCollapses(t *testing.T) {
	t.Parallel()
	rows := buildRows(recs(4, record.LevelInfo), groupThreshold)
	if len(rows) != 1 {
		t.Fatalf("rows: got %d want 1", len(rows))
	}
	if rows[0].kind != rowGroup || len(rows[0].group) != 4 {
		t.Fatalf("expected one group of 4, got %+v", rows[0])
	}
}

func TestBuildRowsMixedLevelsBreakRun(t *testing.T) {
	t.Parallel()
	var entries []*record.Record
	entries = append(entries, recs(3, record.LevelInfo)...)
	entries = append(entries, recs(1, record.LevelError)...)
	entries = append(entries, recs(4, record.LevelInfo)...)

	rows := buildRows(entries, groupThreshold)
	if len(rows) != 5 {
		t.Fatalf("rows: got %d want 5: %+v", len(rows), rows)
	}
	for i := 0; i < 3; i++ {
		if rows[i].kind != rowRecord {
			t.Fatalf("row %d: expected rowRecord, got %+v", i, rows[i])
		}
	}
	if rows[3].kind != rowRecord || rows[3].rec.Level != record.LevelError {
		t.Fatalf("row 3: expected the lone ERROR record, got %+v", rows[3])
	}
	if rows[4].kind != rowGroup || len(rows[4].group) != 4 {
		t.Fatalf("row 4: expected group of 4 INFO, got %+v", rows[4])
	}
}

func TestFoldableGatesByLevelAndKeyword(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		rec  *record.Record
		want bool
	}{
		{"plain info", &record.Record{Level: record.LevelInfo, Raw: "listening on :8080"}, true},
		{"plain debug", &record.Record{Level: record.LevelDebug, Raw: "cache hit for key foo"}, true},
		{"plain warn foldable", &record.Record{Level: record.LevelWarn, Raw: "listening on :8080"}, true},
		{"error level excluded regardless of text", &record.Record{Level: record.LevelError, Raw: "listening on :8080"}, false},
		{"info with warning keyword excluded", &record.Record{Level: record.LevelInfo, Raw: "[WARNING] disk usage high"}, false},
		{"debug with failure keyword excluded", &record.Record{Level: record.LevelDebug, Raw: "unexpected failure retrying"}, false},
		{"info with exception keyword excluded", &record.Record{Level: record.LevelInfo, Raw: "caught exception, continuing"}, false},
		{"warn with own warning keyword still foldable", &record.Record{Level: record.LevelWarn, Raw: "[WARNING] disk usage high"}, true},
		{"warn with error-ish keyword excluded", &record.Record{Level: record.LevelWarn, Raw: "exception during cleanup"}, false},
	}
	for _, c := range cases {
		if got := foldable(c.rec); got != c.want {
			t.Errorf("%s: foldable() = %v, want %v", c.name, got, c.want)
		}
	}
}

func TestBuildRowsSplitsRunOnKeywordFlaggedRecord(t *testing.T) {
	t.Parallel()
	before := recs(6, record.LevelInfo)
	flagged := &record.Record{ID: 100, Level: record.LevelInfo, Message: "msg", Raw: "unexpected failure retrying"}
	after := recs(6, record.LevelInfo)
	for i, r := range after {
		r.ID = uint64(200 + i)
	}
	var entries []*record.Record
	entries = append(entries, before...)
	entries = append(entries, flagged)
	entries = append(entries, after...)

	rows := buildRows(entries, groupThreshold)
	if len(rows) != 3 {
		t.Fatalf("rows: got %d want 3: %+v", len(rows), rows)
	}
	if rows[0].kind != rowGroup || len(rows[0].group) != 6 {
		t.Fatalf("row 0: expected group of 6 INFO before the flagged record, got %+v", rows[0])
	}
	if rows[1].kind != rowRecord || rows[1].rec != flagged {
		t.Fatalf("row 1: expected standalone flagged record, got %+v", rows[1])
	}
	if rows[2].kind != rowGroup || len(rows[2].group) != 6 {
		t.Fatalf("row 2: expected group of 6 INFO after the flagged record, got %+v", rows[2])
	}
}

func TestBuildRowsGroupsWarnLevel(t *testing.T) {
	t.Parallel()
	rows := buildRows(recs(4, record.LevelWarn), groupThreshold)
	if len(rows) != 1 {
		t.Fatalf("rows: got %d want 1", len(rows))
	}
	if rows[0].kind != rowGroup || len(rows[0].group) != 4 {
		t.Fatalf("expected one group of 4 WARN, got %+v", rows[0])
	}
}

func TestBuildRowsWarnWithErrorishTextStaysStandalone(t *testing.T) {
	t.Parallel()
	before := recs(6, record.LevelWarn)
	flagged := &record.Record{ID: 100, Level: record.LevelWarn, Message: "msg", Raw: "exception during cleanup"}
	after := recs(6, record.LevelWarn)
	for i, r := range after {
		r.ID = uint64(200 + i)
	}
	var entries []*record.Record
	entries = append(entries, before...)
	entries = append(entries, flagged)
	entries = append(entries, after...)

	rows := buildRows(entries, groupThreshold)
	if len(rows) != 3 {
		t.Fatalf("rows: got %d want 3: %+v", len(rows), rows)
	}
	if rows[0].kind != rowGroup || len(rows[0].group) != 6 {
		t.Fatalf("row 0: expected group of 6 WARN before the flagged record, got %+v", rows[0])
	}
	if rows[1].kind != rowRecord || rows[1].rec != flagged {
		t.Fatalf("row 1: expected standalone flagged record, got %+v", rows[1])
	}
	if rows[2].kind != rowGroup || len(rows[2].group) != 6 {
		t.Fatalf("row 2: expected group of 6 WARN after the flagged record, got %+v", rows[2])
	}
}

func TestBuildRowsGroupPreservesOrderAndMembers(t *testing.T) {
	t.Parallel()
	entries := recs(5, record.LevelDebug)
	rows := buildRows(entries, groupThreshold)
	if len(rows) != 1 || rows[0].kind != rowGroup {
		t.Fatalf("expected single group row, got %+v", rows)
	}
	for i, rec := range rows[0].group {
		if rec != entries[i] {
			t.Fatalf("group[%d]: got ID %d want %d", i, rec.ID, entries[i].ID)
		}
	}
}
