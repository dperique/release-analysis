package job_analysis

import (
	"fmt"

	"github.com/spf13/cobra"
)

var analysisOpts struct {
	prowJobUrl string
	payloadUrl string
}

// Create the analysis command
var AnalysisCmd = &cobra.Command{
	Use:   "analysis",
	Short: "Analyze a particular payload or prow job",
	Long:  `View analysis of a payload url or prow job (add more details)...`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		// Your analysis command logic here
		//url := args[0]
		fmt.Println("analysis called")
		// Rest of your code...
	},
}

func NewAnalysisCmd() *cobra.Command {
	AnalysisCmd.Flags().StringVarP(&analysisOpts.prowJobUrl, "prowjob-url", "j", "", "Prowjob url")
	AnalysisCmd.Flags().StringVarP(&analysisOpts.payloadUrl, "payload-url", "p", "", "Payload url")
	return AnalysisCmd
}
