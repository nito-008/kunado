package main

import (
	"fmt"
	"os"

	"github.com/nito-008/kunado/internal/app"
)

func main() {
	if err := app.Run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "kunado:", err)
		os.Exit(1)
	}
}
