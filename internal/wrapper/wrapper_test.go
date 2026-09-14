package wrapper

import (
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/tcontardo/prettylogs/internal/parser"
	"github.com/tcontardo/prettylogs/internal/record"
)

func TestStreamReaderParsesLines(t *testing.T) {
	t.Parallel()
	in := strings.NewReader("INFO: hello\nERROR: boom\n")
	out := make(chan record.Record, 8)
	StreamReader(in, parser.AutoParser{}, out, nil)
	close(out)
	var got []record.Record
	for rec := range out {
		got = append(got, rec)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records", len(got))
	}
	if got[0].Level != record.LevelInfo || got[1].Level != record.LevelError {
		t.Fatalf("levels %q %q", got[0].Level, got[1].Level)
	}
}

func TestStreamReaderHandlesLineOverOldScannerLimit(t *testing.T) {
	t.Parallel()
	big := strings.Repeat("x", 2*1024*1024) // over bufio.Scanner's old 1MB cap
	in := strings.NewReader("INFO: " + big + "\nERROR: after\n")
	out := make(chan record.Record, 8)
	StreamReader(in, parser.AutoParser{}, out, nil)
	close(out)
	var got []record.Record
	for rec := range out {
		got = append(got, rec)
	}
	if len(got) != 2 {
		t.Fatalf("got %d records, want 2 (the long line should not silently end the stream)", len(got))
	}
	if got[1].Level != record.LevelError {
		t.Fatalf("record after the long line: level %q, want ERROR", got[1].Level)
	}
}

func TestStartCommandCapturesStdout(t *testing.T) {
	t.Parallel()
	out := make(chan record.Record, 8)
	w, err := StartCommand("echo", []string{"INFO: from echo"}, parser.AutoParser{}, out)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := w.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	close(out)
	var got []record.Record
	for rec := range out {
		got = append(got, rec)
	}
	if len(got) != 1 {
		t.Fatalf("got %d records: %+v", len(got), got)
	}
	if !strings.Contains(got[0].Message, "from echo") {
		t.Fatalf("message %q", got[0].Message)
	}
	code, ok := w.ExitCode()
	if !ok || code != 0 {
		t.Fatalf("exit %d ok=%v", code, ok)
	}
}

func TestHasListeningPortFalseWhenExited(t *testing.T) {
	t.Parallel()
	if Exited(1).HasListeningPort() {
		t.Fatal("exited wrapper should not report a listening port")
	}
	if Live().HasListeningPort() {
		t.Fatal("wrapper with no process should not report a listening port")
	}
}

func TestStopTerminatesProcess(t *testing.T) {
	t.Parallel()
	out := make(chan record.Record, 8)
	w, err := StartCommand("sleep", []string{"30"}, parser.AutoParser{}, out)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if err := w.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	select {
	case <-w.done:
	case <-time.After(2 * time.Second):
		t.Fatal("process did not exit after Stop")
	}
}

func TestStopReturnsWhenOutIsUndrained(t *testing.T) {
	t.Parallel()
	out := make(chan record.Record) // unbuffered, nobody ever reads it
	w, err := StartCommand("sh", []string{"-c", "echo INFO: one; sleep 30"}, parser.AutoParser{}, out)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	// Give the reader goroutine time to read the line and block trying to
	// send it on the channel nothing drains.
	time.Sleep(200 * time.Millisecond)
	start := time.Now()
	done := make(chan error, 1)
	go func() { done <- w.Stop() }()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("stop: %v", err)
		}
	case <-time.After(stopGrace + 3*time.Second):
		t.Fatal("Stop did not return; a reader goroutine is likely stuck sending on the undrained channel")
	}
	if elapsed := time.Since(start); elapsed < stopGrace {
		t.Fatalf("Stop returned after %s, want it to wait out the grace period before force-unblocking readers", elapsed)
	}
}

func TestStopReturnsPromptlyForWellBehavedChild(t *testing.T) {
	t.Parallel()
	out := make(chan record.Record, 8)
	w, err := StartCommand("sleep", []string{"30"}, parser.AutoParser{}, out)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	start := time.Now()
	if err := w.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 2*time.Second {
		t.Fatalf("Stop took %s, want a prompt return after SIGTERM", elapsed)
	}
	if _, ok := w.ExitCode(); !ok {
		t.Fatal("process should have exited after Stop")
	}
}

func TestStopKillsChildThatIgnoresSIGTERM(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM ignore escalation is a unix behavior")
	}
	out := make(chan record.Record, 8)
	w, err := StartCommand("python3", []string{"-c", "import signal,time; signal.signal(signal.SIGTERM, signal.SIG_IGN); print('INFO: ready', flush=True); print('INFO: armed', flush=True); time.sleep(60)"}, parser.AutoParser{}, out)
	if err != nil {
		t.Skipf("python3: %v", err)
	}
	collectN(t, out, 1)
	start := time.Now()
	if err := w.Stop(); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if elapsed := time.Since(start); elapsed > 8*time.Second {
		t.Fatalf("Stop took %s, want escalate-and-return within the grace window", elapsed)
	}
	if _, ok := w.ExitCode(); !ok {
		t.Fatal("process should be gone after Stop escalated to SIGKILL")
	}
}

func TestRestartDeliversLaterOutputOnSameChannel(t *testing.T) {
	t.Parallel()
	out := make(chan record.Record, 8)
	w, err := StartCommand("sh", []string{"-c", "echo INFO: run-a; echo INFO: hold; sleep 30"}, parser.AutoParser{}, out)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	first := collectN(t, out, 1)
	if !strings.Contains(first[0].Message, "run-a") {
		t.Fatalf("first run message %q", first[0].Message)
	}
	if err := w.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case rec := <-out:
			if strings.Contains(rec.Message, "run-a") {
				if err := w.Stop(); err != nil {
					t.Fatalf("stop: %v", err)
				}
				return
			}
		case <-deadline:
			t.Fatal("restarted run never delivered output on the same channel")
		}
	}
}

func TestAbandonPreventsRestart(t *testing.T) {
	t.Parallel()
	out := make(chan record.Record, 8)
	w, err := StartCommand("sh", []string{"-c", "echo INFO: first"}, parser.AutoParser{}, out)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	first := collectN(t, out, 1)
	if !strings.Contains(first[0].Message, "first") {
		t.Fatalf("first run message %q", first[0].Message)
	}
	if err := w.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	w.Abandon()
	if err := w.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	select {
	case rec := <-out:
		t.Fatalf("restart should not run after Abandon, got %+v", rec)
	case <-time.After(300 * time.Millisecond):
	}
}

func TestRestartAfterExitStartsAgain(t *testing.T) {
	t.Parallel()
	out := make(chan record.Record, 8)
	w, err := StartCommand("sh", []string{"-c", "echo INFO: once"}, parser.AutoParser{}, out)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	first := collectN(t, out, 1)
	if !strings.Contains(first[0].Message, "once") {
		t.Fatalf("first run message %q", first[0].Message)
	}
	if err := w.Wait(); err != nil {
		t.Fatalf("wait: %v", err)
	}
	if err := w.Restart(); err != nil {
		t.Fatalf("restart: %v", err)
	}
	second := collectN(t, out, 1)
	if !strings.Contains(second[0].Message, "once") {
		t.Fatalf("restarted run message %q", second[0].Message)
	}
}

func collectN(t *testing.T, out <-chan record.Record, n int) []record.Record {
	t.Helper()
	got := make([]record.Record, 0, n)
	deadline := time.After(2 * time.Second)
	for len(got) < n {
		select {
		case rec := <-out:
			got = append(got, rec)
		case <-deadline:
			t.Fatalf("got %d records, want %d: %+v", len(got), n, got)
		}
	}
	return got
}
