# TODO: harden wrapped-process shutdown on quit

## Current behavior

Pressing `q` or `Ctrl+C` calls `quit()` in `internal/tui/update.go`, which
calls `Wrapper.Stop()` (`internal/wrapper/wrapper.go`) before returning
`tea.Quit`:

```go
func (m Model) quit() (tea.Model, tea.Cmd) {
	if m.wrapper != nil {
		_ = m.wrapper.Stop()
	}
	return m, tea.Quit
}
```

`Stop()` signals the wrapped process via `killProcess()`:

- **Unix** (`internal/wrapper/proc_unix.go`): the child is started with
  `Setpgid: true`, and `killProcess` sends `SIGTERM` to the whole process
  group (`-pgid`), falling back to `SIGKILL` only if the `SIGTERM` syscall
  itself returns an error. Targeting the group generally reaches
  subprocesses too (e.g. `npx` → `nx` → the actual dev server), as long as
  none of them detached into their own session.
- **Non-unix** (`internal/wrapper/proc_other.go`): `killProcess` is just
  `cmd.Process.Kill()` — a hard kill of the **direct child only**.

## Two gaps found

1. **Fire-and-forget, no wait.** `Stop()` sends the signal and returns
   immediately. Nothing waits for the process to actually exit before
   `main()` returns and the whole `prettylogs` process terminates. Usually
   fine in practice, but not guaranteed — main.go's `run()` only calls
   `w.Stop()` again in the *error* path of `prog.Run()`, and even that
   doesn't wait either.
2. **No SIGTERM→SIGKILL escalation on timeout.** The fallback to `SIGKILL`
   only fires if the `SIGTERM` syscall call itself fails — not if the
   process is alive but ignores/traps `SIGTERM`. A stubborn wrapped process
   (or a signal-handling child) can linger indefinitely as an orphan after
   `prettylogs` has already quit.
3. **(Non-unix only) No process-tree kill.** `cmd.Process.Kill()` only
   terminates the direct child; if it spawned its own subprocesses, they're
   left running.

## Suggested fix

### 1. Wait-with-timeout-then-escalate, on all platforms

Change `Wrapper.Stop()` (`internal/wrapper/wrapper.go`) to actually wait for
`w.done` (already closed by the existing background goroutine once
`cmd.Wait()` returns) up to some deadline (e.g. 3–5s) after sending the
initial signal, escalating to a hard kill if the deadline passes:

```go
func (w *Wrapper) Stop() error {
	if w == nil || w.cmd == nil {
		return nil
	}
	select {
	case <-w.done:
		return nil
	default:
	}
	if err := killProcess(w.cmd); err != nil {
		return err
	}
	select {
	case <-w.done:
		return nil
	case <-time.After(5 * time.Second):
		return forceKillProcess(w.cmd)
	}
}
```

Split `killProcess` (graceful: `SIGTERM`/group on unix, or a graceful
signal-equivalent on Windows) from a new `forceKillProcess` (hard:
`SIGKILL`/group on unix, `taskkill /T /F` or Windows job-object termination
on Windows — see below).

Then have `quit()` in `update.go` actually wait on the result — likely via
a `tea.Cmd` that runs `Stop()` and returns a message when done, rather than
calling it synchronously inline, so the TUI can (optionally) show a brief
"shutting down…" state instead of blocking the event loop for up to 5s.
`main.go`'s `run()` should also just rely on this rather than its own
separate `w.Stop()` call in the error path.

### 2. Process-tree kill on Windows

`internal/wrapper/proc_other.go`'s `killProcess`/new `forceKillProcess`
should kill the whole subprocess tree, not just the direct child. Options:

- Use a Windows job object (`CreateJobObject` +
  `AssignProcessToJobObject` on the child at `cmd.Start()` time, then
  `TerminateJobObject` to kill the whole tree) — the "correct" native
  approach, but needs `golang.org/x/sys/windows` and some cgo-free syscall
  plumbing.
- Simpler/cruder alternative: shell out to `taskkill /PID <pid> /T /F`,
  which kills the process and its children by PID tree. Less clean but much
  less code.

### 3. Tests

- `internal/wrapper/wrapper_test.go` already exists — add a test that
  starts a long-running child (or a test helper binary) that ignores
  `SIGTERM`, calls `Stop()`, and asserts it's actually gone (via `SIGKILL`
  escalation) within the timeout window rather than hanging forever.
- A test confirming `Stop()` returns promptly (not blocking `main()`
  indefinitely) when the child exits cleanly on the first signal.

## Not urgent because

In the common case (well-behaved child processes that exit promptly on
`SIGTERM`, on macOS/Linux), current behavior already works correctly — this
is about closing edge cases (stubborn processes, Windows subprocess trees),
not a fix for a reported failure.
