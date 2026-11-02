//go:build !windows

package platform

func AvailableMemory() uint64 { return 0 }
