package job_analysis

import (
	"fmt"

	"github.com/spf13/cobra"
)

var analysisOpts struct {
	url string
	addDetails bool
}

// Create the analysis command
var AnalysisCmd = &cobra.Command{
	Use:   "analysis",
	Short: "Analyze a particular payload or prow job",
	Long:  `View analysis of a payload url or prow job (add more details)...`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("analysis called")
		analysisOpts.url = args[0]
		fmt.Println("url:", analysisOpts.url)
		fmt.Println("addDetails:", analysisOpts.addDetails)
	},
}

func NewAnalysisCmd() *cobra.Command {
	AnalysisCmd.Flags().BoolVarP(&analysisOpts.addDetails, "add_details", "a", false, "Payload url")
	return AnalysisCmd
}
