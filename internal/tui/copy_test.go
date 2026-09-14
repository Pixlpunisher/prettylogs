package tui

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tcontardo/prettylogs/internal/record"
)

// copyTS builds a local timestamp so Format("15:04:05") is stable in assertions.
func copyTS(hour, min, sec int) time.Time {
	return time.Date(2026, 9, 9, hour, min, sec, 0, time.Local)
}

func TestFormatCopyEmptyRow(t *testing.T) {
	t.Parallel()
	r := row{} // zero value is rowRecord with rec == nil
	if got := formatCopy(r, false); got != "" {
		t.Fatalf("pretty: got %q want empty", got)
	}
	if got := formatCopy(r, true); got != "" {
		t.Fatalf("raw: got %q want empty", got)
	}
}

func TestFormatCopySinglePretty(t *testing.T) {
	t.Parallel()
	r := row{kind: rowRecord, rec: &record.Record{
		Level:     record.LevelError,
		Timestamp: copyTS(10, 23, 45),
		Source:    "auth.service.ts:124",
		Message:   "Database connection failed",
		Raw:       `{"level":"error","msg":"Database connection failed"}`,
	}}
	want := "ERROR  10:23:45  auth.service.ts:124\nDatabase connection failed"
	if got := formatCopy(r, false); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatCopySingleRaw(t *testing.T) {
	t.Parallel()
	raw := `{"level":"error","msg":"Database connection failed"}`
	r := row{kind: rowRecord, rec: &record.Record{
		Level:     record.LevelError,
		Timestamp: copyTS(10, 23, 45),
		Source:    "auth.service.ts:124",
		Message:   "Database connection failed",
		Raw:       raw,
	}}
	if got := formatCopy(r, true); got != raw {
		t.Fatalf("got %q want %q", got, raw)
	}
}

func TestFormatCopyExtrasSorted(t *testing.T) {
	t.Parallel()
	r := row{kind: rowRecord, rec: &record.Record{
		Level:     record.LevelError,
		Timestamp: copyTS(10, 23, 45),
		Source:    "auth.service.ts:124",
		Message:   "Database connection failed",
		// Inserted out of order on purpose: pretty output must still be requestId then userId.
		Extra: map[string]string{
			"userId":    "42",
			"requestId": "abc-123",
		},
	}}
	want := "ERROR  10:23:45  auth.service.ts:124\nDatabase connection failed\n\nrequestId: abc-123\nuserId: 42"
	if got := formatCopy(r, false); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatCopyMultilineMessage(t *testing.T) {
	t.Parallel()
	r := row{kind: rowRecord, rec: &record.Record{
		Level:   record.LevelError,
		Message: "boom\n  at foo.ts:1",
	}}
	want := "ERROR\nboom\n  at foo.ts:1"
	if got := formatCopy(r, false); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatCopyShortProjectStack(t *testing.T) {
	t.Parallel()
	r := row{kind: rowRecord, rec: &record.Record{
		Level:   record.LevelError,
		Message: "boom\n  at foo.ts:1\n  at bar.ts:2\n  at baz.ts:3",
	}}
	want := "ERROR\nboom\n  at foo.ts:1\n  at bar.ts:2\n  at baz.ts:3"
	if got := formatCopy(r, false); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatCopyCollapsesLibraryFrameRuns(t *testing.T) {
	t.Parallel()
	r := row{kind: rowRecord, rec: &record.Record{
		Level: record.LevelError,
		Message: strings.Join([]string{
			"NullPointerException: boom",
			"	at com.example.api.ApiApplication.main(ApiApplication.java:18)",
			"	at org.springframework.boot.SpringApplication.run(SpringApplication.java:1)",
			"	at org.springframework.boot.SpringApplication.run(SpringApplication.java:2)",
			"	at org.springframework.boot.SpringApplication.run(SpringApplication.java:3)",
			"	at org.springframework.boot.SpringApplication.run(SpringApplication.java:4)",
			"Caused by: java.net.BindException: Address already in use",
			"	at org.apache.tomcat.util.net.NioEndpoint.bind(NioEndpoint.java:1)",
			"	at org.apache.tomcat.util.net.NioEndpoint.bind(NioEndpoint.java:2)",
			"	at org.apache.tomcat.util.net.NioEndpoint.bind(NioEndpoint.java:3)",
		}, "\n"),
	}}
	want := strings.Join([]string{
		"ERROR",
		"NullPointerException: boom",
		"	at com.example.api.ApiApplication.main(ApiApplication.java:18)",
		"    … 4 library frames omitted …",
		"Caused by: java.net.BindException: Address already in use",
		"    … 3 library frames omitted …",
	}, "\n")
	if got := formatCopy(r, false); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatCopyHeadTailNonFrameWall(t *testing.T) {
	t.Parallel()
	n := prettyBodyMaxLines + 10
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i)
	}
	r := row{kind: rowRecord, rec: &record.Record{
		Level:   record.LevelError,
		Message: strings.Join(lines, "\n"),
	}}
	omitted := n - prettyBodyHeadLines - prettyBodyTailLines
	wantLines := append([]string{"ERROR"}, lines[:prettyBodyHeadLines]...)
	wantLines = append(wantLines, fmt.Sprintf("… %d lines omitted …", omitted))
	wantLines = append(wantLines, lines[n-prettyBodyTailLines:]...)
	want := strings.Join(wantLines, "\n")
	if got := formatCopy(r, false); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatCopyMultilineStackExtra(t *testing.T) {
	t.Parallel()
	stack := strings.Join([]string{
		"Error: boom",
		"    at Pool.connect (/src/db/pool.ts:45)",
		"    at node_modules/pg/lib/client.js:1",
		"    at node_modules/pg/lib/client.js:2",
		"    at node_modules/pg/lib/client.js:3",
	}, "\n")
	r := row{kind: rowRecord, rec: &record.Record{
		Level:   record.LevelError,
		Message: "Database connection failed",
		Extra:   map[string]string{"stack": stack},
	}}
	want := strings.Join([]string{
		"ERROR",
		"Database connection failed",
		"",
		"stack:",
		"Error: boom",
		"    at Pool.connect (/src/db/pool.ts:45)",
		"    … 3 library frames omitted …",
	}, "\n")
	if got := formatCopy(r, false); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatCopySkipsDuplicateStackExtra(t *testing.T) {
	t.Parallel()
	msg := "boom\n  at foo.ts:1"
	r := row{kind: rowRecord, rec: &record.Record{
		Level:   record.LevelError,
		Message: msg,
		Extra:   map[string]string{"stack": msg},
	}}
	want := "ERROR\nboom\n  at foo.ts:1"
	if got := formatCopy(r, false); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatCopyOmitsEmptySourceAndZeroTime(t *testing.T) {
	t.Parallel()
	r := row{kind: rowRecord, rec: &record.Record{
		Level:   record.LevelInfo,
		Message: "listening on :8080",
	}}
	want := "INFO\nlistening on :8080"
	if got := formatCopy(r, false); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestFormatCopyGroupPrettyAndRaw(t *testing.T) {
	t.Parallel()
	msgs := []string{"msg-0", "msg-1", "msg-2", "msg-3"}
	raws := []string{"raw-0", "raw-1", "raw-2", "raw-3"}
	group := make([]*record.Record, 4)
	for i := range group {
		group[i] = &record.Record{
			Level:     record.LevelInfo,
			Timestamp: copyTS(10, 23, 45+i),
			Message:   msgs[i],
			Raw:       raws[i],
		}
	}
	r := row{kind: rowGroup, group: group}

	wantPretty := "INFO ×4  10:23:45 → 10:23:48\n\n10:23:45  msg-0\n10:23:46  msg-1\n10:23:47  msg-2\n10:23:48  msg-3"
	if got := formatCopy(r, false); got != wantPretty {
		t.Fatalf("pretty: got %q want %q", got, wantPretty)
	}

	wantRaw := "raw-0\nraw-1\nraw-2\nraw-3"
	if got := formatCopy(r, true); got != wantRaw {
		t.Fatalf("raw: got %q want %q", got, wantRaw)
	}
}

func stubClipboard(t *testing.T) *string {
	t.Helper()
	orig := writeClipboard
	t.Cleanup(func() { writeClipboard = orig })
	var got string
	writeClipboard = func(s string) error {
		got = s
		return nil
	}
	return &got
}

func TestCopySelectionPrettyAndRaw(t *testing.T) {
	got := stubClipboard(t)
	raw := `{"level":"error","msg":"Database connection failed"}`
	m := newTestModel(record.Record{
		Level:     record.LevelError,
		Timestamp: copyTS(10, 23, 45),
		Source:    "auth.service.ts:124",
		Message:   "Database connection failed",
		Raw:       raw,
	})

	updated, _ := m.handleKey(keyMsg("y"))
	m = updated.(Model)
	if m.copyStatus != "copied 1 record" {
		t.Fatalf("status: got %q", m.copyStatus)
	}
	wantPretty := "ERROR  10:23:45  auth.service.ts:124\nDatabase connection failed"
	if *got != wantPretty {
		t.Fatalf("clipboard pretty: got %q want %q", *got, wantPretty)
	}
	if !strings.Contains(m.footerView(m.rows()), "copied 1 record") {
		t.Fatalf("footer missing copy status:\n%s", m.footerView(m.rows()))
	}

	updated, _ = m.handleKey(keyMsg("Y"))
	m = updated.(Model)
	if m.copyStatus != "copied 1 record (raw)" {
		t.Fatalf("status raw: got %q", m.copyStatus)
	}
	if *got != raw {
		t.Fatalf("clipboard raw: got %q want %q", *got, raw)
	}
}

func TestCopySelectionFooterCountsOmittedLines(t *testing.T) {
	stubClipboard(t)
	n := prettyBodyMaxLines + 10
	lines := make([]string, n)
	for i := range lines {
		lines[i] = fmt.Sprintf("line-%d", i)
	}
	m := newTestModel(record.Record{
		Level:   record.LevelError,
		Message: strings.Join(lines, "\n"),
		Raw:     "raw",
	})
	updated, _ := m.handleKey(keyMsg("y"))
	m = updated.(Model)
	kept := prettyBodyHeadLines + 1 + prettyBodyTailLines
	want := fmt.Sprintf("copied 1 record (%d of %d lines)", kept, n)
	if m.copyStatus != want {
		t.Fatalf("status: got %q want %q", m.copyStatus, want)
	}
	if !strings.Contains(m.footerView(m.rows()), want) {
		t.Fatalf("footer missing copy status:\n%s", m.footerView(m.rows()))
	}

	updated, _ = m.handleKey(keyMsg("Y"))
	m = updated.(Model)
	if m.copyStatus != "copied 1 record (raw)" {
		t.Fatalf("raw should not show line counts, got %q", m.copyStatus)
	}
}

func TestCopySelectionNothingToCopy(t *testing.T) {
	called := false
	orig := writeClipboard
	t.Cleanup(func() { writeClipboard = orig })
	writeClipboard = func(string) error {
		called = true
		return nil
	}
	m := newTestModel()
	updated, _ := m.handleKey(keyMsg("y"))
	m = updated.(Model)
	if m.copyStatus != "nothing to copy" {
		t.Fatalf("status: got %q", m.copyStatus)
	}
	if called {
		t.Fatal("clipboard should not be written when the list is empty")
	}
}

func TestCopySelectionWriteError(t *testing.T) {
	orig := writeClipboard
	t.Cleanup(func() { writeClipboard = orig })
	writeClipboard = func(string) error { return errors.New("no display") }
	m := newTestModel(record.Record{Level: record.LevelInfo, Message: "hi", Raw: "hi"})
	updated, _ := m.handleKey(keyMsg("y"))
	m = updated.(Model)
	if m.copyStatus != "copy failed: no display" {
		t.Fatalf("status: got %q", m.copyStatus)
	}
}

func TestCopySelectionClearsOnMove(t *testing.T) {
	stubClipboard(t)
	m := newTestModel(
		record.Record{Level: record.LevelError, Message: "a", Raw: "a"},
		record.Record{Level: record.LevelError, Message: "b", Raw: "b"},
	)
	updated, _ := m.handleKey(keyMsg("y"))
	m = updated.(Model)
	if m.copyStatus == "" {
		t.Fatal("expected copy status after y")
	}
	updated, _ = m.handleKey(keyMsg("j"))
	m = updated.(Model)
	if m.copyStatus != "" {
		t.Fatalf("status should clear on move, got %q", m.copyStatus)
	}
}

func TestCopySelectionDoesNotRunWhileSearching(t *testing.T) {
	got := stubClipboard(t)
	m := newTestModel(record.Record{Level: record.LevelInfo, Message: "hi", Raw: "hi"})
	updated, _ := m.handleKey(keyMsg("/"))
	m = updated.(Model)
	updated, _ = m.handleKey(keyMsg("y"))
	m = updated.(Model)
	if m.copyStatus != "" {
		t.Fatalf("searching should type y, not copy; status=%q", m.copyStatus)
	}
	if *got != "" {
		t.Fatalf("clipboard written while searching: %q", *got)
	}
}
