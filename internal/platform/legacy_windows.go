//go:build windows

package platform

import (
	"encoding/binary"
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"strings"
)

// shortPath returns an existing Windows short name when the volume provides one.
func shortPath(path string) string {
	if path == "" {
		return path
	}
	input, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return path
	}
	buffer := make([]uint16, 1024)
	for {
		n, err := windows.GetShortPathName(input, &buffer[0], uint32(len(buffer)))
		if err != nil || n == 0 {
			return path
		}
		if n >= uint32(len(buffer)) {
			buffer = make([]uint16, n+1)
			continue
		}
		return windows.UTF16ToString(buffer[:n])
	}
}

func asciiPath(path string) bool {
	for _, r := range path {
		if r > 127 {
			return false
		}
	}
	return true
}

// Alias the common media root, rather than just the working directory, so
// relative references such as ../../assets remain inside the same tree.
func LegacyWorkingDir(dir string, related ...string) (string, func(), error) {
	noop := func() {}
	// A UTF-8 manifest on the Go executable does not change a child helper's
	// ANSI code page. Protect non-ASCII paths independently of our own GetACP.
	if asciiPath(dir) {
		return dir, noop, nil
	}
	if short := shortPath(dir); asciiPath(short) {
		return short, noop, nil
	}
	root := dir
	for _, path := range related {
		for {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return "", noop, err
			}
			if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				break
			}
			parent := filepath.Dir(root)
			if parent == root {
				return "", noop, fmt.Errorf("authoring assets must share a local volume")
			}
			root = parent
		}
	}
	tempRoot := shortPath(os.TempDir())
	if !asciiPath(tempRoot) {
		return "", noop, fmt.Errorf("DVD helpers need an ASCII temporary path on this Windows locale; set TEMP to a local folder")
	}
	parent, err := os.MkdirTemp(tempRoot, "easydvd-path-")
	if err != nil {
		return "", noop, err
	}
	link := filepath.Join(parent, "media")
	cleanup := func() { _ = os.Remove(link); _ = os.Remove(parent) }
	if err = directoryJunction(link, root); err != nil {
		cleanup()
		return "", noop, err
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		cleanup()
		return "", noop, err
	}
	return filepath.Join(link, rel), cleanup, nil
}

// Directory junction creation uses Unicode APIs and requires no symlink privilege.
// Cleanup removes only the junction itself, never its target directory.
func directoryJunction(link, target string) error {
	if err := os.Mkdir(link, 0700); err != nil {
		return err
	}
	absolute, err := filepath.Abs(target)
	if err != nil {
		return err
	}
	substitute := `\??\` + absolute
	if strings.HasPrefix(absolute, `\\`) {
		substitute = `\??\UNC\` + strings.TrimPrefix(absolute, `\\`)
	}
	sub, err := windows.UTF16FromString(substitute)
	if err != nil {
		return err
	}
	printName, err := windows.UTF16FromString(absolute)
	if err != nil {
		return err
	}
	names := append(sub, printName...)
	buffer := make([]byte, 16+2*len(names))
	if len(buffer) > 16*1024 {
		return fmt.Errorf("legacy authoring path is too long")
	}
	binary.LittleEndian.PutUint32(buffer[0:], windows.IO_REPARSE_TAG_MOUNT_POINT)
	binary.LittleEndian.PutUint16(buffer[4:], uint16(len(buffer)-8))
	binary.LittleEndian.PutUint16(buffer[10:], uint16((len(sub)-1)*2))
	binary.LittleEndian.PutUint16(buffer[12:], uint16(len(sub)*2))
	binary.LittleEndian.PutUint16(buffer[14:], uint16((len(printName)-1)*2))
	for i, code := range names {
		binary.LittleEndian.PutUint16(buffer[16+2*i:], code)
	}
	path, err := windows.UTF16PtrFromString(link)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(path, windows.GENERIC_WRITE, windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING, windows.FILE_FLAG_OPEN_REPARSE_POINT|windows.FILE_FLAG_BACKUP_SEMANTICS, 0)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(handle)
	var returned uint32
	return windows.DeviceIoControl(handle, windows.FSCTL_SET_REPARSE_POINT, &buffer[0], uint32(len(buffer)), nil, 0, &returned, nil)
}
