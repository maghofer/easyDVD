//go:build windows

package platform

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// AvailableMemory returns currently available physical memory, or zero if the
// system query is unavailable. It is used only to bound automatic concurrency.
func AvailableMemory() uint64 {
	var status struct {
		Length, Load                                       uint32
		TotalPhys, AvailPhys, TotalPageFile, AvailPageFile uint64
		TotalVirtual, AvailVirtual, AvailExtendedVirtual   uint64
	}
	status.Length = uint32(unsafe.Sizeof(status))
	ok, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GlobalMemoryStatusEx").Call(uintptr(unsafe.Pointer(&status)))
	if ok == 0 {
		return 0
	}
	return status.AvailPhys
}
