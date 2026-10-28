//go:build !windows

package platform

import "syscall"

func IsLocal(path string) bool { return true }

func FreeBytes(path string) (uint64, error) {
	var s syscall.Statfs_t
	err := syscall.Statfs(path, &s)
	return s.Bavail * uint64(s.Bsize), err
}
