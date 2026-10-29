//go:build !windows

package platform

func LegacyWorkingDir(path string, related ...string) (string, func(), error) {
	return path, func() {}, nil
}
