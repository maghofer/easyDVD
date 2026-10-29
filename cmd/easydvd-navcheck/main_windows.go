//go:build windows

// easydvd-navcheck validates a three-title, two-titles-per-page navigation fixture.
package main

import (
	"fmt"
	"golang.org/x/sys/windows"
	"os"
	"path/filepath"
	"runtime"
	"time"
	"unsafe"
)

func main() {
	if len(os.Args) != 3 && !(len(os.Args) == 4 && os.Args[3] == "--autoplay") {
		fmt.Fprintln(os.Stderr, "Usage: easydvd-navcheck VLC_DIRECTORY FIXTURE.iso [--autoplay] (otherwise: 3 titles, 2 menu entries per page)")
		os.Exit(2)
	}
	root, _ := filepath.Abs(os.Args[1])
	p, _ := windows.UTF16PtrFromString(root)
	windows.NewLazySystemDLL("kernel32.dll").NewProc("SetDllDirectoryW").Call(uintptr(unsafe.Pointer(p)))
	os.Setenv("VLC_PLUGIN_PATH", filepath.Join(root, "plugins"))
	dll := windows.NewLazyDLL(filepath.Join(root, "libvlc.dll"))
	call := func(name string, args ...uintptr) uintptr { r, _, _ := dll.NewProc(name).Call(args...); return r }
	strings := []string{"--intf=dummy", "--vout=dummy", "--aout=dummy", "--avcodec-hw=none", "--ignore-config", "--no-media-library", "--no-video-title-show", "--quiet", "--file-caching=100", "--disc-caching=100"}
	var ptrs []*byte
	for _, s := range strings {
		b, _ := windows.BytePtrFromString(s)
		ptrs = append(ptrs, b)
	}
	instance := call("libvlc_new", uintptr(len(ptrs)), uintptr(unsafe.Pointer(&ptrs[0])))
	runtime.KeepAlive(ptrs)
	if instance == 0 {
		panic("vlc init failed")
	}
	defer call("libvlc_release", instance)
	path, _ := filepath.Abs(os.Args[2])
	uri := "dvd:///" + filepath.ToSlash(path)
	b, _ := windows.BytePtrFromString(uri)
	media := call("libvlc_media_new_location", instance, uintptr(unsafe.Pointer(b)))
	runtime.KeepAlive(b)
	defer call("libvlc_media_release", media)
	option, _ := windows.BytePtrFromString(":avcodec-hw=none")
	call("libvlc_media_add_option", media, uintptr(unsafe.Pointer(option)))
	runtime.KeepAlive(option)
	player := call("libvlc_media_player_new_from_media", media)
	defer call("libvlc_media_player_release", player)
	defer call("libvlc_media_player_stop", player)
	title := func() int { return int(int32(call("libvlc_media_player_get_title", player))) }
	state := func() int { return int(call("libvlc_media_player_get_state", player)) }
	waitTitle := func(want int, timeout time.Duration) {
		end := time.Now().Add(timeout)
		for time.Now().Before(end) {
			if title() == want {
				return
			}
			time.Sleep(30 * time.Millisecond)
		}
		panic(fmt.Sprintf("wanted title %d, got %d state %d", want, title(), state()))
	}
	nav := func(n int) {
		call("libvlc_media_player_navigate", player, uintptr(n))
		time.Sleep(120 * time.Millisecond)
	}
	call("libvlc_media_player_play", player)
	if len(os.Args) == 4 {
		waitTitle(1, 5*time.Second)
		fmt.Println("Automatic first play: title", title())
		end := time.Now().Add(15 * time.Second)
		for time.Now().Before(end) {
			if state() == 6 {
				fmt.Println("Playback ended without a menu; PASS")
				return
			}
			time.Sleep(30 * time.Millisecond)
		}
		panic(fmt.Sprintf("autoplay did not finish, title %d state %d", title(), state()))
	}
	waitTitle(0, 5*time.Second)
	time.Sleep(500 * time.Millisecond)
	fmt.Println("Initial menu: title", title(), "chapter", int32(call("libvlc_media_player_get_chapter", player)))
	nav(0)
	waitTitle(1, 3*time.Second)
	fmt.Println("Selected title 1")
	waitTitle(0, 5*time.Second)
	fmt.Println("Returned to menu")
	nav(2)
	nav(2)
	nav(0)
	waitTitle(1, 3*time.Second)
	fmt.Println("Play All title 1")
	waitTitle(2, 5*time.Second)
	fmt.Println("Play All title 2")
	waitTitle(3, 5*time.Second)
	fmt.Println("Play All title 3")
	waitTitle(0, 5*time.Second)
	fmt.Println("Play All returned to menu")
	nav(2)
	nav(2)
	nav(2)
	nav(0)
	time.Sleep(300 * time.Millisecond)
	fmt.Println("Next page: title", title(), "chapter", int32(call("libvlc_media_player_get_chapter", player)))
	nav(0)
	waitTitle(3, 3*time.Second)
	fmt.Println("Selected title 3 on page 2")
	waitTitle(0, 5*time.Second)
	fmt.Println("Returned to page 2")
	fmt.Println("PASS")
}
