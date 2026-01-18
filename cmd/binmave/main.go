package main

import (
	"os"

	"github.com/Binmave/binmave-cli/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
