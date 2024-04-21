package main

import (
	"os"

	"github.com/dperique/release-analysis/releaseanalysiscommands"
)

func main() {
	cmd := releaseanalysiscommands.CreateRelContCommand()
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
