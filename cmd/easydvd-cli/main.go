package main

import (
	"easydvd/internal/app"
	"easydvd/internal/platform"
	"fmt"
	"os"
)

func main() {
	release, err := platform.HoldInstallerMutex()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer release()
	if err := app.CLI(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
