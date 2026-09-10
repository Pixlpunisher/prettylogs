package wrapper

import (
	"net"
	"os"
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
	StreamReader(in, parser.AutoParser{}, out)
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

func TestPidHasListeningPort(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	if !pidHasListeningPort(os.Getpid()) {
		t.Fatal("expected this process to report a listening TCP port")
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
