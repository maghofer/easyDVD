//go:build windows

package platform

import (
	"fmt"
	"golang.org/x/sys/windows"
	"runtime"
	"syscall"
	"unsafe"
)

// PickFolders uses the native Windows multi-folder dialog on a dedicated COM STA.
func PickFolders(multiple bool) ([]string, error) {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole := windows.NewLazySystemDLL("ole32.dll")
	init := ole.NewProc("CoInitializeEx")
	uninit := ole.NewProc("CoUninitialize")
	create := ole.NewProc("CoCreateInstance")
	free := ole.NewProc("CoTaskMemFree")
	hr, _, _ := init.Call(0, 2)
	if int32(hr) < 0 {
		return nil, fmt.Errorf("initialize folder picker: 0x%x", hr)
	}
	defer uninit.Call()
	clsid := windows.GUID{Data1: 0xdc1c5a9c, Data2: 0xe88a, Data3: 0x4dde, Data4: [8]byte{0xa5, 0xa1, 0x60, 0xf8, 0x2a, 0x20, 0xae, 0xf7}}
	iid := windows.GUID{Data1: 0xd57c7288, Data2: 0xd4ad, Data3: 0x4768, Data4: [8]byte{0xbe, 0x02, 0x9d, 0x96, 0x95, 0x32, 0xd9, 0x60}}
	var dlg *comObject
	hr, _, _ = create.Call(uintptr(unsafe.Pointer(&clsid)), 0, 1, uintptr(unsafe.Pointer(&iid)), uintptr(unsafe.Pointer(&dlg)))
	if int32(hr) < 0 {
		return nil, fmt.Errorf("create folder picker: 0x%x", hr)
	}
	defer comCall(dlg, 2)
	var flags uint32
	comCall(dlg, 10, uintptr(unsafe.Pointer(&flags)))
	flags |= 0x20 | 0x40 | 0x800 | 0x8
	if multiple {
		flags |= 0x200
	}
	comCall(dlg, 9, uintptr(flags))
	title, _ := windows.UTF16PtrFromString("Add folders to easyDVD")
	comCall(dlg, 17, uintptr(unsafe.Pointer(title)))
	hr = comCall(dlg, 3, 0)
	if uint32(hr) == 0x800704c7 {
		return nil, nil
	}
	if int32(hr) < 0 {
		return nil, fmt.Errorf("folder picker: 0x%x", hr)
	}
	var arr *comObject
	hr = comCall(dlg, 27, uintptr(unsafe.Pointer(&arr)))
	if int32(hr) < 0 {
		return nil, fmt.Errorf("read selected folders: 0x%x", hr)
	}
	defer comCall(arr, 2)
	var count uint32
	comCall(arr, 7, uintptr(unsafe.Pointer(&count)))
	var paths []string
	for n := uint32(0); n < count; n++ {
		var item *comObject
		hr = comCall(arr, 8, uintptr(n), uintptr(unsafe.Pointer(&item)))
		if int32(hr) < 0 {
			return nil, fmt.Errorf("read folder %d", n)
		}
		var p *uint16
		hr = comCall(item, 5, 0x80058000, uintptr(unsafe.Pointer(&p)))
		if int32(hr) >= 0 {
			paths = append(paths, windows.UTF16PtrToString(p))
			free.Call(uintptr(unsafe.Pointer(p)))
		}
		comCall(item, 2)
		if int32(hr) < 0 {
			return nil, fmt.Errorf("selected item is not a filesystem folder")
		}
	}
	return paths, nil
}

type comObject struct{ table *[29]uintptr }

func comCall(obj *comObject, slot int, args ...uintptr) uintptr {
	fn := obj.table[slot]
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{uintptr(unsafe.Pointer(obj))}, args...)...)
	return r
}
