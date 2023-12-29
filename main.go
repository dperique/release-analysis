package main

import (
	"os"

	"github.com/dperique/release-analysis/cmd"
)

func main() {
	cmd := cmd.CreateRelContCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
