package payload

import (
	"fmt"

	"github.com/spf13/cobra"
)

var payloadOpts struct {
	version           string
	stream            string
	showAllUrl        bool
	showAggrTimes     bool
	showSuccess       bool
	dbMode            string
	printTestDetail   bool
	showAggrJobDetail bool
}

// Create the payload command
var PayloadCmd = &cobra.Command{
	Use:   "payload aVersion aStream",
	Short: "View payload of release-controller given a Version (e.g., 4.15, 4.16) and a Stream (e.g., nightly, ci)",
	Long:  `View payload of release-controller given a Version (e.g., 4.15, 4.16) and a Stream (e.g., nightly, ci)`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		payloadOpts.version = args[0]
		payloadOpts.stream = args[1]
		fmt.Println("version:", payloadOpts.version)
		fmt.Println("stream:", payloadOpts.stream)
		fmt.Println("showAllUrl:", payloadOpts.showAllUrl)
		fmt.Println("showAggrTimes:", payloadOpts.showAggrTimes)
		fmt.Println("showSuccess:", payloadOpts.showSuccess)
		fmt.Println("dbMode:", payloadOpts.dbMode)
		fmt.Println("printTestDetail:", payloadOpts.printTestDetail)
		fmt.Println("showAggrJobDetail:", payloadOpts.showAggrJobDetail)
	},
}

func NewPayloadCmd() *cobra.Command {
	PayloadCmd.Flags().BoolVarP(&payloadOpts.showAllUrl, "showAllUrl", "a", true, "Show all url")
	PayloadCmd.Flags().BoolVarP(&payloadOpts.showAggrTimes, "showAggrTimes", "s", true, "Show aggregated times")
	PayloadCmd.Flags().BoolVarP(&payloadOpts.showSuccess, "showSuccess", "c", false, "Show success")
	PayloadCmd.Flags().StringVarP(&payloadOpts.dbMode, "dbMode", "d", "rcWebpage", "DB mode (rcWebpage, sippyDB, rcAPI)")
	PayloadCmd.Flags().BoolVarP(&payloadOpts.printTestDetail, "printTestDetail", "t", false, "Print test detail")
	PayloadCmd.Flags().BoolVarP(&payloadOpts.showAggrJobDetail, "showAggrJobDetail", "j", false, "Show aggregated job detail")
	return PayloadCmd
}
