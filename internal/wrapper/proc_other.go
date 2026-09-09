//go:build !unix

package wrapper

import "os/exec"

func setProcAttr(cmd *exec.Cmd) {}

func killProcess(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}
	return cmd.Process.Kill()
}
