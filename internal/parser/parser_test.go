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

func TestPlainParserBracketedWarningTag(t *testing.T) {
	t.Parallel()
	line := `[WARNING] For this reason, future Maven versions might no longer support building such malformed projects.`
	rec := parsePlain(line)
	if rec.Level != record.LevelWarn {
		t.Fatalf("level: got %q want WARN", rec.Level)
	}
	if rec.Message != "For this reason, future Maven versions might no longer support building such malformed projects." {
		t.Fatalf("message: got %q", rec.Message)
	}
}

func TestPlainParserBracketedErrorTag(t *testing.T) {
	t.Parallel()
	line := `[ERROR] something broke`
	rec := parsePlain(line)
	if rec.Level != record.LevelError {
		t.Fatalf("level: got %q want ERROR", rec.Level)
	}
	if rec.Message != "something broke" {
		t.Fatalf("message: got %q", rec.Message)
	}
}

func TestInferFrontendLevelDetectsWarnFallback(t *testing.T) {
	t.Parallel()
	rec := parsePlain("retrying after warn from upstream, attempt 2")
	if rec.Level != record.LevelWarn {
		t.Fatalf("level: got %q want WARN (inferFrontendLevel fallback)", rec.Level)
	}
}

func TestPlainParserRecognizesPanicAndCritical(t *testing.T) {
	t.Parallel()
	if rec := parsePlain("panic: index out of range [5] with length 3"); rec.Level != record.LevelError {
		t.Fatalf("panic level: got %q want ERROR", rec.Level)
	}
	if rec := parsePlain("CRITICAL: disk full"); rec.Level != record.LevelError {
		t.Fatalf("critical level: got %q want ERROR", rec.Level)
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

func TestAssemblerIgnoresWhitespaceOnlyLines(t *testing.T) {
	t.Parallel()
	a := NewAssembler(AutoParser{})
	var got []record.Record
	got = append(got, a.Feed("ERROR: boom")...)
	got = append(got, a.Feed("   ")...)
	got = append(got, a.Feed("INFO: next thing")...)
	got = append(got, a.Flush()...)
	if len(got) != 2 {
		t.Fatalf("records: got %d want 2", len(got))
	}
	if got[0].Raw != "ERROR: boom" {
		t.Fatalf("whitespace leaked into block: %q", got[0].Raw)
	}
}

func TestAssemblerDoesNotSwallowIndentedInfoLines(t *testing.T) {
	t.Parallel()
	a := NewAssembler(AutoParser{})
	var got []record.Record
	// nx/npm indent grouped subprocess output for cosmetic alignment; none
	// of these are stack-trace continuations and must stay separate records.
	got = append(got, a.Feed("  [Nest] 1  - LOG [NestFactory] Starting Nest application...")...)
	got = append(got, a.Feed("  [Nest] 1  - LOG [InstanceLoader] AppModule dependencies initialized")...)
	got = append(got, a.Feed("  [Nest] 1  - LOG [NestApplication] Nest application successfully started")...)
	got = append(got, a.Flush()...)
	if len(got) != 3 {
		t.Fatalf("records: got %d want 3 (indented INFO lines got merged): %+v", len(got), got)
	}
}

func TestAssemblerStripsANSICodes(t *testing.T) {
	t.Parallel()
	a := NewAssembler(AutoParser{})
	got := a.Feed("\x1b[32mINFO\x1b[0m: server started")
	got = append(got, a.Flush()...)
	if len(got) != 1 {
		t.Fatalf("records: got %d want 1", len(got))
	}
	if strings.Contains(got[0].Raw, "\x1b") {
		t.Fatalf("ANSI escape leaked into block: %q", got[0].Raw)
	}
	if got[0].Level != record.LevelInfo {
		t.Fatalf("level: got %q want INFO", got[0].Level)
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

func feedAll(lines ...string) []record.Record {
	a := NewAssembler(AutoParser{})
	var got []record.Record
	for _, line := range lines {
		got = append(got, a.Feed(line)...)
	}
	return append(got, a.Flush()...)
}

func TestAssemblerJoinsSpringFailureAnalysis(t *testing.T) {
	t.Parallel()
	got := feedAll(
		"10:55:49.682 [main] ERROR org.springframework.boot.diagnostics.LoggingFailureAnalysisReporter --",
		"***************************",
		"APPLICATION FAILED TO START",
		"***************************",
		"Description:",
		"Web server failed to start. Port 8080 was already in use.",
	)
	if len(got) != 1 {
		t.Fatalf("records: got %d want 1: %+v", len(got), got)
	}
	if got[0].Level != record.LevelError {
		t.Fatalf("level: got %q want ERROR", got[0].Level)
	}
	if !strings.Contains(got[0].Raw, "APPLICATION FAILED TO START") {
		t.Fatalf("title missing from raw: %q", got[0].Raw)
	}
	if !strings.Contains(got[0].Raw, "Web server failed to start") {
		t.Fatalf("description missing from raw: %q", got[0].Raw)
	}
}

func TestAssemblerKeepsTwoErrorHeadersSeparate(t *testing.T) {
	t.Parallel()
	got := feedAll("ERROR: first failed", "ERROR: second failed")
	if len(got) != 2 {
		t.Fatalf("records: got %d want 2: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "first failed") {
		t.Fatalf("first message: %q", got[0].Message)
	}
	if !strings.Contains(got[1].Message, "second failed") {
		t.Fatalf("second message: %q", got[1].Message)
	}
	if strings.Contains(got[0].Raw, "second failed") {
		t.Fatalf("second error was merged into the first: %q", got[0].Raw)
	}
}

func TestAssemblerStartsNewErrorOnLogHeader(t *testing.T) {
	t.Parallel()
	got := feedAll(
		"ERROR: reporter --",
		"***************************",
		"APPLICATION FAILED TO START",
		"10:55:50.001 [main] ERROR o.s.boot.SpringApplication -- Application run failed",
	)
	if len(got) != 2 {
		t.Fatalf("records: got %d want 2: %+v", len(got), got)
	}
	if got[0].Level != record.LevelError || got[1].Level != record.LevelError {
		t.Fatalf("levels: %q %q", got[0].Level, got[1].Level)
	}
	if !strings.Contains(got[0].Raw, "APPLICATION FAILED TO START") {
		t.Fatalf("banner should stay on the first error: %q", got[0].Raw)
	}
	if !strings.Contains(got[1].Message, "Application run failed") && !strings.Contains(got[1].Raw, "Application run failed") {
		t.Fatalf("second header should be its own record: %q", got[1].Raw)
	}
}

func TestAssemblerJoinsSeparatorOntoInfo(t *testing.T) {
	t.Parallel()
	got := feedAll("INFO: starting", "==========")
	if len(got) != 1 {
		t.Fatalf("records: got %d want 1: %+v", len(got), got)
	}
	if got[0].Level != record.LevelInfo {
		t.Fatalf("level: got %q want INFO", got[0].Level)
	}
	if !strings.Contains(got[0].Raw, "==========") {
		t.Fatalf("separator not attached: %q", got[0].Raw)
	}
}

func TestCompilePhase(t *testing.T) {
	t.Parallel()
	cases := []struct {
		line      string
		wantStart bool
		wantEnd   bool
	}{
		{line: "Compiling...", wantStart: true},
		{line: "Starting compilation", wantStart: true},
		{line: "rebuild started", wantStart: true},
		{line: "Compiled successfully in 4.2s", wantEnd: true},
		{line: "event - compiled client and server successfully in 2.3s", wantEnd: true},
		{line: "ready - started server on 0.0.0.0:3000", wantEnd: true},
		{line: "listening on :8080", wantEnd: true},
		{line: "  ➜  Local: http://localhost:5173/", wantEnd: true},
		{line: "INFO: hello", wantStart: false, wantEnd: false},
	}
	for _, tc := range cases {
		start, end := CompilePhase(tc.line)
		if start != tc.wantStart || end != tc.wantEnd {
			t.Fatalf("%q: start=%v end=%v want start=%v end=%v", tc.line, start, end, tc.wantStart, tc.wantEnd)
		}
	}
}

func TestAssemblerJoinsMavenReactorNoiseOntoError(t *testing.T) {
	t.Parallel()
	got := feedAll(
		"12:53:25.623 [main] ERROR org.springframework.boot.diagnostics.LoggingFailureAnalysisReporter --",
		"[INFO] ------------------------------------------------------------------------",
		"BUILD FAILURE",
		"[INFO] Total time:  24.533 s",
		"[INFO] Finished at: 2026-09-10T12:53:25+02:00",
	)
	if len(got) != 1 {
		t.Fatalf("records: got %d want 1: %+v", len(got), got)
	}
	if got[0].Level != record.LevelError {
		t.Fatalf("level: got %q want ERROR", got[0].Level)
	}
	if !strings.Contains(got[0].Raw, "BUILD FAILURE") {
		t.Fatalf("BUILD FAILURE missing from raw: %q", got[0].Raw)
	}
}

func TestAssemblerKeepsMavenGoalErrorSeparate(t *testing.T) {
	t.Parallel()
	got := feedAll(
		"12:53:25.623 [main] ERROR org.springframework.boot.diagnostics.LoggingFailureAnalysisReporter --",
		"[INFO] ------------------------------------------------------------------------",
		"BUILD FAILURE",
		"[INFO] Total time:  24.533 s",
		"[INFO] Finished at: 2026-09-10T12:53:25+02:00",
		"[ERROR] Failed to execute goal org.springframework.boot:spring-boot-maven-plugin:3.5.16:run (default-cli) on project teaching-aid-service-api: Process terminated with exit code: 1 -> [Help 1]",
	)
	if len(got) != 2 {
		t.Fatalf("records: got %d want 2: %+v", len(got), got)
	}
	if strings.Contains(got[0].Raw, "Failed to execute goal") {
		t.Fatalf("goal error was merged into the Spring record: %q", got[0].Raw)
	}
	if !strings.Contains(got[1].Raw, "Failed to execute goal") {
		t.Fatalf("goal error missing from second record: %q", got[1].Raw)
	}
}

func TestAssemblerJoinsMavenHelpOntoGoalError(t *testing.T) {
	t.Parallel()
	got := feedAll(
		"12:53:25.623 [main] ERROR org.springframework.boot.diagnostics.LoggingFailureAnalysisReporter --",
		"[INFO] ------------------------------------------------------------------------",
		"BUILD FAILURE",
		"[ERROR] Failed to execute goal org.springframework.boot:spring-boot-maven-plugin:3.5.16:run (default-cli) on project teaching-aid-service-api: Process terminated with exit code: 1 -> [Help 1]",
		"[ERROR]",
		"[ERROR] To see the full stack trace of the errors, re-run Maven with the -e switch.",
		"[ERROR] Re-run Maven using the -X switch to enable full debug logging.",
		"[ERROR] For more information about the errors and possible solutions, please read the following articles:",
		"[Help 1] http://cwiki.apache.org/confluence/display/MAVEN/MojoExecutionException",
	)
	if len(got) != 2 {
		t.Fatalf("records: got %d want 2: %+v", len(got), got)
	}
	if !strings.Contains(got[1].Raw, "To see the full stack trace") {
		t.Fatalf("help text should be on the goal record: %q", got[1].Raw)
	}
	if !strings.Contains(got[1].Raw, "[Help 1]") {
		t.Fatalf("Help 1 should be on the goal record: %q", got[1].Raw)
	}
}

func TestAssemblerSplitsNxEpilogueFromMavenError(t *testing.T) {
	t.Parallel()
	got := feedAll(
		"12:53:25.623 [main] ERROR org.springframework.boot.diagnostics.LoggingFailureAnalysisReporter --",
		"[INFO] ------------------------------------------------------------------------",
		"BUILD FAILURE",
		"[ERROR] Failed to execute goal org.springframework.boot:spring-boot-maven-plugin:3.5.16:run (default-cli) on project teaching-aid-service-api: Process terminated with exit code: 1 -> [Help 1]",
		"[ERROR] To see the full stack trace of the errors, re-run Maven with the -e switch.",
		"[Help 1] http://cwiki.apache.org/confluence/display/MAVEN/MojoExecutionException",
		" NX   Running target serve-with-local-config for project teaching-aid-service-api and 2 tasks it depends on failed",
		"Failed tasks:",
		"- teaching-aid-service-api:serve-with-local-config",
	)
	if len(got) != 3 {
		t.Fatalf("records: got %d want 3: %+v", len(got), got)
	}
	if got[2].Level != record.LevelError {
		t.Fatalf("nx record level: got %q want ERROR", got[2].Level)
	}
	if strings.Contains(got[1].Raw, "NX") {
		t.Fatalf("nx epilogue was glued onto the goal record: %q", got[1].Raw)
	}
	if !strings.Contains(got[2].Raw, "Running target") {
		t.Fatalf("nx banner missing from third record: %q", got[2].Raw)
	}
	if !strings.Contains(got[2].Raw, "Failed tasks:") {
		t.Fatalf("Failed tasks should stay on the nx record: %q", got[2].Raw)
	}
}

func TestAssemblerDoesNotTreatMavenHelpURLAsNx(t *testing.T) {
	t.Parallel()
	got := feedAll(
		"[ERROR] Failed to execute goal org.springframework.boot:spring-boot-maven-plugin:3.5.16:run (default-cli) on project teaching-aid-service-api: Process terminated with exit code: 1 -> [Help 1]",
		"[Help 1] http://example.com/NX-something",
		"Failed tasks:",
		"- teaching-aid-service-api:serve-with-local-config",
	)
	if len(got) != 2 {
		t.Fatalf("records: got %d want 2: %+v", len(got), got)
	}
	if strings.Contains(got[0].Raw, "Failed tasks:") {
		t.Fatalf("Failed tasks was glued onto the Maven help record: %q", got[0].Raw)
	}
	if !strings.Contains(got[1].Raw, "Failed tasks:") {
		t.Fatalf("Failed tasks should start its own record: %q", got[1].Raw)
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
