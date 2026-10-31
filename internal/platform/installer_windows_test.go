//go:build windows

package platform

import (
	"fmt"
	"os"
	"testing"

	"golang.org/x/sys/windows"
)

func TestInstallerMarkerRemainsUntilLastProcessHandleCloses(t *testing.T) {
	name := fmt.Sprintf(`Local\easyDVD.InstallerTest.%d`, os.Getpid())
	p, err := windows.UTF16PtrFromString(name)
	if err != nil {
		t.Fatal(err)
	}
	first, err := holdInstallerMutex(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first)
	second, err := holdInstallerMutex(name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(second)
	first()
	h, err := windows.OpenMutex(windows.SYNCHRONIZE, false, p)
	if err != nil {
		t.Fatalf("closing one instance removed the marker for another: %v", err)
	}
	windows.CloseHandle(h)
	second()
	h, err = windows.OpenMutex(windows.SYNCHRONIZE, false, p)
	if err != windows.ERROR_FILE_NOT_FOUND {
		if err == nil {
			windows.CloseHandle(h)
		}
		t.Fatalf("marker survived the last instance: %v", err)
	}
}
