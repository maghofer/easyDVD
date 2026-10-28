//go:build !windows

package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "The libVLC navigation fixture currently runs on Windows only.")
	os.Exit(1)
}
