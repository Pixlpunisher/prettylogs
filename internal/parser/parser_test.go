package parser

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tcontardo/prettylogs/internal/record"
)

func testdata(t *testing.T, name string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..", "testdata", "samples", name)
}

func readLines(t *testing.T, path string) []string {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open %s: %v", path, err)
	}
	defer f.Close()
	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan %s: %v", path, err)
	}
	return lines
}

func TestJSONParserExtractsFields(t *testing.T) {
	t.Parallel()
	line := `{"level":"error","time":"2026-09-09T10:23:45Z","msg":"Database connection failed","file":"auth.service.ts","line":124}`
	rec, ok := (JSONParser{}).Parse(line)
	if !ok {
		t.Fatal("expected JSON parse to succeed")
	}
	if rec.Level != record.LevelError {
		t.Fatalf("level: got %q want ERROR", rec.Level)
	}
	if rec.Message != "Database connection failed" {
		t.Fatalf("message: got %q", rec.Message)
	}
	if rec.Source != "auth.service.ts:124" {
		t.Fatalf("source: got %q", rec.Source)
	}
	want := time.Date(2026, 9, 9, 10, 23, 45, 0, time.UTC)
	if !rec.Timestamp.Equal(want) {
		t.Fatalf("timestamp: got %v want %v", rec.Timestamp, want)
	}
}

func TestLogfmtParserQuotedMessage(t *testing.T) {
	t.Parallel()
	line := `level=warn ts=2026-09-09T10:23:44Z msg="Redis reconnection attempt 3/5" file=cache.service.ts line=89`
	rec, ok := (LogfmtParser{}).Parse(line)
	if !ok {
		t.Fatal("expected logfmt parse to succeed")
	}
	if rec.Level != record.LevelWarn {
		t.Fatalf("level: got %q", rec.Level)
	}
	if rec.Message != "Redis reconnection attempt 3/5" {
		t.Fatalf("message: got %q", rec.Message)
	}
	if rec.Source != "cache.service.ts:89" {
		t.Fatalf("source: got %q", rec.Source)
	}
}

func TestSyslogParserRFC5424(t *testing.T) {
	t.Parallel()
	line := `<27>1 2026-09-09T10:23:45Z host app 12 - - boom`
	rec, ok := (SyslogParser{}).Parse(line)
	if !ok {
		t.Fatal("expected syslog parse to succeed")
	}
	if rec.Level != record.LevelError {
		t.Fatalf("level: got %q want ERROR (severity 3)", rec.Level)
	}
	if rec.Message != "boom" {
		t.Fatalf("message: got %q", rec.Message)
	}
	if rec.Source != "app" {
		t.Fatalf("source: got %q", rec.Source)
	}
}

func TestPlainParserLevelPrefixAndSource(t *testing.T) {
	t.Parallel()
	line := `2026-09-09 10:23:45 ERROR Database connection failed auth.service.ts:124`
	rec := parsePlain(line)
	if rec.Level != record.LevelError {
		t.Fatalf("level: got %q", rec.Level)
	}
	if !strings.Contains(rec.Message, "Database connection failed") {
		t.Fatalf("message: got %q", rec.Message)
	}
	if rec.Source != "auth.service.ts:124" {
		t.Fatalf("source: got %q", rec.Source)
	}
}

func TestAutoParserDetectsJSONOverLogfmt(t *testing.T) {
	t.Parallel()
	line := `{"level":"info","msg":"hi=there"}`
	rec, ok := (AutoParser{}).Parse(line)
	if !ok {
		t.Fatal("expected auto parse to succeed")
	}
	if rec.Message != "hi=there" {
		t.Fatalf("wanted JSON message, got %q", rec.Message)
	}
}

func TestFrontendPatternsAreInfo(t *testing.T) {
	t.Parallel()
	cases := []string{
		"ready - started server on 0.0.0.0:3000, url: http://localhost:3000",
		"event - compiled client and server successfully in 2.3s",
		"[vite] connecting...",
		"Compiled successfully in 4.2s",
		"Compiling...",
	}
	for _, line := range cases {
		rec := parsePlain(line)
		if rec.Level != record.LevelInfo {
			t.Fatalf("%q: level %q want INFO", line, rec.Level)
		}
	}
}

func TestAssemblerJoinsStackTrace(t *testing.T) {
	t.Parallel()
	a := NewAssembler(AutoParser{})
	var got []record.Record
	got = append(got, a.Feed("ERROR: Module not found ./src/missing.ts")...)
	got = append(got, a.Feed("  at Object.<anonymous> (/src/app/page.tsx:12)")...)
	got = append(got, a.Feed("INFO: recovered")...)
	got = append(got, a.Flush()...)
	if len(got) != 2 {
		t.Fatalf("records: got %d want 2", len(got))
	}
	if !strings.Contains(got[0].Raw, "at Object") {
		t.Fatalf("stack not attached: %q", got[0].Raw)
	}
	if got[1].Level != record.LevelInfo {
		t.Fatalf("second record level: %q", got[1].Level)
	}
}

func TestForFormatRejectsUnknown(t *testing.T) {
	t.Parallel()
	if _, err := ForFormat("xml"); err == nil {
		t.Fatal("expected error for unknown format")
	}
}

func TestSampleFilesParseAllLines(t *testing.T) {
	t.Parallel()
	files := []string{"json.log", "logfmt.log", "plain.log", "nextjs.log", "vite.log"}
	for _, name := range files {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			a := NewAssembler(AutoParser{})
			var recs []record.Record
			for _, line := range readLines(t, testdata(t, name)) {
				recs = append(recs, a.Feed(line)...)
			}
			recs = append(recs, a.Flush()...)
			if len(recs) == 0 {
				t.Fatal("expected at least one record")
			}
			for _, rec := range recs {
				if rec.Message == "" {
					t.Fatalf("empty message in %q raw=%q", name, rec.Raw)
				}
			}
		})
	}
}

func TestJSONParserRejectsNonJSON(t *testing.T) {
	t.Parallel()
	if _, ok := (JSONParser{}).Parse("ERROR: not json"); ok {
		t.Fatal("expected JSON parser to reject plain text")
	}
}

func TestLogfmtParserRejectsPlain(t *testing.T) {
	t.Parallel()
	if _, ok := (LogfmtParser{}).Parse("no pairs here"); ok {
		t.Fatal("expected logfmt parser to reject plain text")
	}
}
