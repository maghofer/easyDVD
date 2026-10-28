//go:build !windows

package platform

import (
	"os/exec"
	"runtime"
)

func OpenFolder(path string) error {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	return exec.Command(command, path).Run()
}
