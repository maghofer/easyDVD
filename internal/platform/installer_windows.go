//go:build windows

package platform

import (
	"fmt"
	"sync"

	"golang.org/x/sys/windows"
)

// HoldInstallerMutex marks this process as active for Inno Setup. It deliberately
// permits multiple processes: Windows keeps the object alive until the last
// handle is closed, including while the engine is stopping its child processes.
func HoldInstallerMutex() (func(), error) {
	return holdInstallerMutex(`Global\easyDVD.Running`)
}

func holdInstallerMutex(mutexName string) (func(), error) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return nil, err
	}
	h, err := windows.CreateMutex(nil, false, name)
	if err != nil && err != windows.ERROR_ALREADY_EXISTS {
		return nil, fmt.Errorf("create installer activity marker: %w", err)
	}
	var once sync.Once
	return func() { once.Do(func() { _ = windows.CloseHandle(h) }) }, nil
}
