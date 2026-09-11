package wrapper

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/tcontardo/prettylogs/internal/parser"
	"github.com/tcontardo/prettylogs/internal/record"
)

const stopGrace = 5 * time.Second

type Wrapper struct {
	mu       sync.Mutex
	cmd      *exec.Cmd
	done     chan struct{}
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

func StreamReader(r io.Reader, p parser.Parser, out chan<- record.Record) {
	asm := parser.NewAssembler(p)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		for _, rec := range asm.Feed(sc.Text()) {
			out <- rec
		}
	}
	for _, rec := range asm.Flush() {
		out <- rec
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
	w.mu.Lock()
	w.cmd = cmd
	w.done = done
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
		StreamReader(stdout, p, out)
	}()
	go func() {
		defer wg.Done()
		StreamReader(stderr, p, out)
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
	w.mu.Lock()
	if w.abandon {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()
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
