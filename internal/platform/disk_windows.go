//go:build windows

package platform

import (
	"golang.org/x/sys/windows"
	"path/filepath"
)

func IsLocal(path string) bool {
	root := filepath.VolumeName(path) + `\`
	p, err := windows.UTF16PtrFromString(root)
	if err != nil {
		return false
	}
	return windows.GetDriveType(p) != windows.DRIVE_REMOTE
}

func FreeBytes(path string) (uint64, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return 0, err
	}
	var free, total, all uint64
	err = windows.GetDiskFreeSpaceEx(p, &free, &total, &all)
	return free, err
}
