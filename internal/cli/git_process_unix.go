//go:build !windows

package cli

import (
	"os/exec"
	"syscall"
)

func configureCommandGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func attachCommandGroup(cmd *exec.Cmd) error {
	return nil
}

func killCommandGroup(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	_ = cmd.Process.Kill()
}

func cleanupCommandGroup(cmd *exec.Cmd) {}
