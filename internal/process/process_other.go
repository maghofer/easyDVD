//go:build !windows

package process

import (
	"os/exec"
	"syscall"
)

func prepare(cmd *exec.Cmd) (func(), error) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	return func() {}, nil
}
func attach(cmd *exec.Cmd) error { return nil }
