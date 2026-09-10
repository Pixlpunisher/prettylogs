package wrapper

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/tcontardo/prettylogs/internal/parser"
	"github.com/tcontardo/prettylogs/internal/record"
)

type Wrapper struct {
	cmd      *exec.Cmd
	done     chan struct{}
	waitErr  error
	exitOnce sync.Once
	code     int
	exited   bool
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
	cmd := exec.Command(name, args...)
	setProcAttr(cmd)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	devNull, err := os.OpenFile(os.DevNull, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	cmd.Stdin = devNull
	if err := cmd.Start(); err != nil {
		_ = devNull.Close()
		return nil, err
	}

	w := &Wrapper{cmd: cmd, done: make(chan struct{})}
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
		w.waitErr = cmd.Wait()
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
		close(w.done)
	}()
	return w, nil
}

func (w *Wrapper) Wait() error {
	if w == nil {
		return nil
	}
	<-w.done
	return w.waitErr
}

func (w *Wrapper) Stop() error {
	if w == nil || w.cmd == nil {
		return nil
	}
	select {
	case <-w.done:
		return nil
	default:
		return killProcess(w.cmd)
	}
}

func (w *Wrapper) ExitCode() (int, bool) {
	if w == nil {
		return 0, false
	}
	select {
	case <-w.done:
		return w.code, true
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
	if w == nil || w.cmd == nil || w.cmd.Process == nil {
		return false
	}
	select {
	case <-w.done:
		return false
	default:
	}
	return groupHasListeningPort(w.cmd.Process.Pid)
}
