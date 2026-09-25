//go:build unix

package cmd

import (
	"os/exec"
	"syscall"
)

// isolateProcessGroup starts cmd in its own process group and makes
// cancellation kill the whole group, so an analyzer that forks (a CLI agent,
// a shell script) doesn't leave children running after its task is cancelled.
func isolateProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error {
		return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
