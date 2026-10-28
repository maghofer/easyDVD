//go:build !windows

package platform

import "fmt"

func PickFolders(multiple bool) ([]string, error) {
	return nil, fmt.Errorf("native folder selection is currently available on Windows; drag folders into the window instead")
}
