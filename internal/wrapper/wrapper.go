package wrapper

import (
	"bufio"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

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
	streamReaderNamed("unknown", r, p, out)
}

func streamReaderNamed(src string, r io.Reader, p parser.Parser, out chan<- record.Record) {
	asm := parser.NewAssembler(p)
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var lines, recs, blocked int64
	var lastLine string
	for sc.Scan() {
		line := sc.Text()
		lines++
		lastLine = line
		if lines <= 5 {
			// #region agent log
			agentLog("E", "wrapper.go:streamReaderNamed", "early line", map[string]any{"src": src, "n": lines, "len": len(line), "preview": trimPreview(line, 120)})
			// #endregion
		}
		for _, rec := range asm.Feed(line) {
			recs++
			sendRec(out, rec, src, &blocked)
		}
	}
	for _, rec := range asm.Flush() {
		recs++
		sendRec(out, rec, src, &blocked)
	}
	// #region agent log
	agentLog("A", "wrapper.go:streamReaderNamed", "stream exit", map[string]any{"src": src, "lines": lines, "recs": recs, "blockedSends": blocked, "scanErr": scanErr(sc), "last": trimPreview(lastLine, 120)})
	// #endregion
}

func sendRec(out chan<- record.Record, rec record.Record, src string, blocked *int64) {
	start := time.Now()
	out <- rec
	d := time.Since(start)
	if d > 50*time.Millisecond {
		*blocked++
		// #region agent log
		agentLog("A", "wrapper.go:sendRec", "channel send blocked", map[string]any{"src": src, "ms": d.Milliseconds(), "blockedSends": *blocked, "preview": trimPreview(rec.Message, 80)})
		// #endregion
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
	// #region agent log
	pid := 0
	if cmd.Process != nil {
		pid = cmd.Process.Pid
	}
	agentLog("D", "wrapper.go:StartCommand", "child started", map[string]any{"pid": pid, "name": name, "args": args, "stdin": "devnull"})
	// #endregion
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		streamReaderNamed("stdout", stdout, p, out)
	}()
	go func() {
		defer wg.Done()
		streamReaderNamed("stderr", stderr, p, out)
	}()
	go pollChild(w, pid)
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
		// #region agent log
		errStr := ""
		if w.waitErr != nil {
			errStr = w.waitErr.Error()
		}
		agentLog("D", "wrapper.go:StartCommand", "child wait returned", map[string]any{"pid": pid, "code": w.code, "err": errStr})
		// #endregion
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

// #region agent log
func agentLog(hid, loc, msg string, data map[string]any) {
	f, err := os.OpenFile("/Users/tcontardo/Github/PrettyLogs/.cursor/debug-c9d99c.log", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	_ = json.NewEncoder(f).Encode(map[string]any{
		"sessionId":    "c9d99c",
		"hypothesisId": hid,
		"location":     loc,
		"message":      msg,
		"data":         data,
		"timestamp":    time.Now().UnixMilli(),
	})
}

func trimPreview(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", "\\n")
	if len(s) > n {
		return s[:n]
	}
	return s
}

func scanErr(sc *bufio.Scanner) string {
	if err := sc.Err(); err != nil {
		return err.Error()
	}
	return ""
}

func pollChild(w *Wrapper, pid int) {
	start := time.Now()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for i := 0; i < 20; i++ {
		select {
		case <-w.done:
			agentLog("C", "wrapper.go:pollChild", "poll stop, child done", map[string]any{"pid": pid, "elapsedMs": time.Since(start).Milliseconds(), "ticks": i})
			return
		case <-ticker.C:
			agentLog("C", "wrapper.go:pollChild", "process tree", map[string]any{
				"pid":       pid,
				"elapsedMs": time.Since(start).Milliseconds(),
				"tick":      i,
				"tree":      processTree(pid),
			})
		}
	}
}

func processTree(pid int) []map[string]string {
	var out []map[string]string
	seen := map[int]bool{}
	var walk func(int)
	walk = func(id int) {
		if id <= 0 || seen[id] {
			return
		}
		seen[id] = true
		info := procInfo(id)
		out = append(out, info)
		for _, child := range pgrepChildren(id) {
			walk(child)
		}
	}
	walk(pid)
	return out
}

func procInfo(pid int) map[string]string {
	out, err := exec.Command("ps", "-o", "pid=,ppid=,state=,command=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return map[string]string{"pid": strconv.Itoa(pid), "state": "gone"}
	}
	fields := strings.Fields(strings.TrimSpace(string(out)))
	info := map[string]string{"pid": strconv.Itoa(pid), "raw": strings.TrimSpace(string(out))}
	if len(fields) >= 3 {
		info["ppid"] = fields[1]
		info["state"] = fields[2]
		if len(fields) > 3 {
			info["cmd"] = strings.Join(fields[3:], " ")
			if len(info["cmd"]) > 80 {
				info["cmd"] = info["cmd"][:80]
			}
		}
	}
	return info
}

func pgrepChildren(pid int) []int {
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(pid)).Output()
	if err != nil {
		return nil
	}
	var kids []int
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if n, err := strconv.Atoi(strings.TrimSpace(line)); err == nil {
			kids = append(kids, n)
		}
	}
	return kids
}

// #endregion
