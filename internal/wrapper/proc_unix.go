//go:build unix

package wrapper

import (
	"os/exec"
	"syscall"
)

func setProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	pgid := cmd.Process.Pid
	if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}
	return nil
}
