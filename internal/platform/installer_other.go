//go:build !windows

package platform

func HoldInstallerMutex() (func(), error) { return func() {}, nil }
