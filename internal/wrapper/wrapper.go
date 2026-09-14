package wrapper

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/tcontardo/prettylogs/internal/parser"
	"github.com/tcontardo/prettylogs/internal/record"
)

const stopGrace = 5 * time.Second

// stopSignal lets Stop() unblock a StreamReader that is stuck sending to an
// undrained out channel (e.g. the TUI quit before consuming everything).
// close is idempotent since Stop() and a force-kill timeout can race.
type stopSignal struct {
	ch   chan struct{}
	once sync.Once
}

func newStopSignal() *stopSignal {
	return &stopSignal{ch: make(chan struct{})}
}

func (s *stopSignal) close() {
	if s == nil {
		return
	}
	s.once.Do(func() { close(s.ch) })
}

type Wrapper struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	done     chan struct{}
	stop     *stopSignal
	waitErr  error
	exitOnce sync.Once
	code     int
	exited   bool
	abandon  bool

	name   string
	args   []string
	parser parser.Parser
	out    chan<- record.Record
}

// StreamReader parses lines from r and feeds them to out until r is
// exhausted, or (if stop is non-nil) until stop is closed. A bufio.Reader is
// used instead of bufio.Scanner because Scanner silently stops at its first
// too-long line (indistinguishable here from a clean EOF); ReadString has no
// such line-length ceiling.
func StreamReader(r io.Reader, p parser.Parser, out chan<- record.Record, stop <-chan struct{}) {
	asm := parser.NewAssembler(p)
	br := bufio.NewReaderSize(r, 64*1024)
	for {
		line, err := br.ReadString('\n')
		if len(line) > 0 {
			for _, rec := range asm.Feed(strings.TrimRight(line, "\r\n")) {
				select {
				case out <- rec:
				case <-stop:
					return
				}
			}
		}
		if err != nil {
			break
		}
	}
	for _, rec := range asm.Flush() {
		select {
		case out <- rec:
		case <-stop:
			return
		}
	}
}

func StartCommand(name string, args []string, p parser.Parser, out chan<- record.Record) (*Wrapper, error) {
	copied := make([]string, len(args))
	copy(copied, args)
	w := &Wrapper{
		name:   name,
		args:   copied,
		parser: p,
		out:    out,
	}
	if err := w.start(); err != nil {
		return nil, err
	}
	return w, nil
}

func (w *Wrapper) start() error {
	cmd := exec.Command(w.name, w.args...)
	setProcAttr(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		return err
	}
	cmd.Stdin = devNull
	if err := cmd.Start(); err != nil {
		_ = devNull.Close()
		return err
	}

	done := make(chan struct{})
	stop := newStopSignal()
	w.mu.Lock()
	if w.abandon {
		// Abandon() raced in between Restart()'s Stop() and this start():
		// discard the process we just spawned instead of wiring it up, so
		// no restarted process outlives an abandonment request.
		w.mu.Unlock()
		_ = killProcess(cmd)
		_ = forceKillProcess(cmd)
		go func() {
			_ = cmd.Wait()
			_ = devNull.Close()
		}()
		return nil
	}
	w.cmd = cmd
	w.done = done
	w.stop = stop
	w.exitOnce = sync.Once{}
	w.exited = false
	w.code = 0
	w.waitErr = nil
	w.mu.Unlock()

	p := w.parser
	out := w.out
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		StreamReader(stdout, p, out, stop.ch)
	}()
	go func() {
		defer wg.Done()
		StreamReader(stderr, p, out, stop.ch)
	}()
	go func() {
		wg.Wait()
		_ = devNull.Close()
		waitErr := cmd.Wait()
		w.mu.Lock()
		w.waitErr = waitErr
		w.exitOnce.Do(func() {
			if w.waitErr == nil {
				w.code = 0
			} else if ee, ok := w.waitErr.(*exec.ExitError); ok {
				w.code = ee.ExitCode()
			} else {
				w.code = 1
			}
			w.exited = true
		})
		w.mu.Unlock()
		close(done)
	}()
	return nil
}

func (w *Wrapper) Wait() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	done := w.done
	w.mu.Unlock()
	if done == nil {
		return nil
	}
	<-done
	w.mu.Lock()
	err := w.waitErr
	w.mu.Unlock()
	return err
}

func (w *Wrapper) Stop() error {
	if w == nil {
		return nil
	}
	w.mu.Lock()
	cmd := w.cmd
	done := w.done
	stop := w.stop
	w.mu.Unlock()
	if cmd == nil || done == nil {
		return nil
	}
	select {
	case <-done:
		return nil
	default:
	}
	_ = killProcess(cmd)
	select {
	case <-done:
		return nil
	case <-time.After(stopGrace):
		_ = forceKillProcess(cmd)
		// The process is dead, but a reader goroutine may still be blocked
		// sending to an undrained out channel; unblock it so done can close
		// instead of waiting on it forever.
		stop.close()
		<-done
		return nil
	}
}

func (w *Wrapper) CanRestart() bool {
	return w != nil && w.name != ""
}

func (w *Wrapper) Abandon() {
	if w == nil {
		return
	}
	w.mu.Lock()
	w.abandon = true
	w.mu.Unlock()
}

func (w *Wrapper) Restart() error {
	if !w.CanRestart() {
		return nil
	}
	if err := w.Stop(); err != nil {
		return err
	}
	// start() re-checks abandon itself right before committing the new
	// process's state, closing the race where Abandon() lands between this
	// call and that commit.
	return w.start()
}

func (w *Wrapper) ExitCode() (int, bool) {
	if w == nil {
		return 0, false
	}
	w.mu.Lock()
	done := w.done
	w.mu.Unlock()
	if done == nil {
		return 0, false
	}
	select {
	case <-done:
		w.mu.Lock()
		code := w.code
		w.mu.Unlock()
		return code, true
	default:
		return 0, false
	}
}

// Exited returns a Wrapper that has already finished with the given exit code.
func Exited(code int) *Wrapper {
	done := make(chan struct{})
	close(done)
	return &Wrapper{done: done, code: code, exited: true}
}

// Live returns a Wrapper that has not finished. For tests and status init.
func Live() *Wrapper {
	return &Wrapper{done: make(chan struct{})}
}

func (w *Wrapper) HasListeningPort() bool {
	if w == nil {
		return false
	}
	w.mu.Lock()
	cmd := w.cmd
	done := w.done
	w.mu.Unlock()
	if cmd == nil || cmd.Process == nil || done == nil {
		return false
	}
	select {
	case <-done:
		return false
	default:
	}
	return groupHasListeningPort(cmd.Process.Pid)
}
