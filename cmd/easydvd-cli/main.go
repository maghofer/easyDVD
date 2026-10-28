package main

import (
	"easydvd/internal/app"
	"fmt"
	"os"
)

func main() {
	if err := app.CLI(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
