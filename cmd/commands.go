package cmd

import (
	"github.com/dperique/release-analysis/job_analysis"
	"github.com/dperique/release-analysis/payload"
	"github.com/spf13/cobra"
)

// CreateRelContCommand adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func CreateRelContCommand() *cobra.Command {

	// Create a root command
	var rootCmd = &cobra.Command{
		Use:   "relCont-again view",
		Short: "view payload or analysis",
		Long:  `We can view payload or analysis of release-controller or prowjobs`,
	}

	rootCmd.AddCommand(payload.NewPayloadCmd())
	rootCmd.AddCommand(job_analysis.NewAnalysisCmd())
	return rootCmd
}
