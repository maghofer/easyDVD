package main

import (
	"easydvd"
	"easydvd/internal/app"
	"easydvd/internal/ui"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) == 2 && (os.Args[1] == "--version" || os.Args[1] == "-version") {
		fmt.Println("easyDVD " + easydvd.Version())
		return
	}
	if ok, err := app.Worker(os.Args[1:]); ok {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if len(os.Args) > 1 && os.Args[1] == "--headless" {
		if err := app.CLI(os.Args[2:]); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		return
	}
	if err := ui.Run(); err != nil {
		ui.ShowStartupError(err)
	}
}
