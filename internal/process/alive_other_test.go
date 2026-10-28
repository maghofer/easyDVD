//go:build !windows

package process

import (
	"fmt"
	"os"
	"strings"
	"syscall"
)

func processAlive(pid int) bool {
	if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid)); err == nil {
		tail := string(b)[strings.LastIndex(string(b), ")")+1:]
		state := strings.Fields(tail)
		if len(state) > 0 && (state[0] == "Z" || state[0] == "X") {
			return false
		}
	}
	return syscall.Kill(pid, 0) == nil
}
