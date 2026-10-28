//go:build windows

package platform

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
)

func OpenFolder(path string) error {
	if !filepath.IsAbs(path) {
		return fmt.Errorf("folder path must be absolute: %s", path)
	}
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("not a folder: %s", path)
	}
	// Packaged desktop hosts can virtualize AppData. Explorer runs outside
	// that context, so give it the final filesystem path from an open handle.
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open folder: %w", err)
	}
	defer f.Close()
	buffer := make([]uint16, 1024)
	for {
		n, resolveErr := windows.GetFinalPathNameByHandle(windows.Handle(f.Fd()), &buffer[0], uint32(len(buffer)), 0)
		if resolveErr != nil {
			return fmt.Errorf("resolve folder: %w", resolveErr)
		}
		if n >= uint32(len(buffer)) {
			buffer = make([]uint16, n+1)
			continue
		}
		path = windows.UTF16ToString(buffer[:n])
		break
	}
	if strings.HasPrefix(path, `\\?\UNC\`) {
		path = `\\` + strings.TrimPrefix(path, `\\?\UNC\`)
	} else {
		path = strings.TrimPrefix(path, `\\?\`)
	}
	verb, _ := windows.UTF16PtrFromString("open")
	target, err := windows.UTF16PtrFromString(filepath.Clean(path))
	if err != nil {
		return err
	}
	if err = windows.ShellExecute(0, verb, target, nil, nil, windows.SW_SHOWNORMAL); err != nil {
		return fmt.Errorf("open folder %s: %w", path, err)
	}
	return nil
}
