package payload

import (
	"fmt"

	"github.com/dperique/release-analysis/payload_processing"
	"github.com/spf13/cobra"
)

type payloadOptsType struct {
	version              string
	stream               string
	showAllUrl           bool
	showAllUrlStr        string
	showAggrTimes        bool
	showAggrTimesStr     string
	showSuccess          bool
	showSuccessStr       string
	dbMode               string
	payload_getter       payload_processing.PayloadGetter
	printTestDetail      bool
	printTestDetailStr   string
	showAggrJobDetail    bool
	showAggrJobDetailStr string
}

var payloadOpts payloadOptsType

// Create the payload command
var PayloadCmd = &cobra.Command{
	Use:   "payload aVersion aStream",
	Short: "View payload of release-controller given a Version (e.g., 4.15, 4.16) and a Stream (e.g., nightly, ci)",
	Long:  `View payload of release-controller given a Version (e.g., 4.15, 4.16) and a Stream (e.g., nightly, ci)`,
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		version := args[0]
		if version != "4.13" && version != "4.14" && version != "4.15" && version != "4.16" {
			fmt.Println("Invalid version. Version must be between 4.13 and 4.16.")
			return
		}
		payloadOpts.version = version
		stream := args[1]
		if stream != "nightly" && stream != "ci" {
			fmt.Println("Invalid stream. Stream must be either 'nightly' or 'ci'.")
			return
		}
		payloadOpts.stream = stream

		payloadOpts.showAllUrl = true
		if payloadOpts.showAllUrlStr == "false" {
			payloadOpts.showAllUrl = false
		}
		payloadOpts.showAggrTimes = true
		if payloadOpts.showAggrTimesStr == "false" {
			payloadOpts.showAggrTimes = false
		}
		if payloadOpts.showSuccessStr == "true" {
			payloadOpts.showSuccess = true
		}
		if payloadOpts.printTestDetailStr == "true" {
			payloadOpts.printTestDetail = true
		}
		if payloadOpts.showAggrJobDetailStr == "true" {
			payloadOpts.showAggrJobDetail = true
		}

		switch payloadOpts.dbMode {
		case "rcWebpage":
			payloadOpts.payload_getter = payload_processing.RcWebpagePayloadGetter{}
		case "sippyDB":
			payloadOpts.payload_getter = payload_processing.SippyDBPayloadGetter{}
		case "rcAPI":
			payloadOpts.payload_getter = payload_processing.RcAPIPayloadGetter{}
		default:
			fmt.Println("Unknown dbMode; defaulting to rcWebpage")
			payloadOpts.payload_getter = payload_processing.RcWebpagePayloadGetter{}
		}
		payloadOpts.Run()
	},
}

func NewPayloadCmd() *cobra.Command {
	PayloadCmd.Flags().StringVarP(&payloadOpts.showAllUrlStr, "showAllUrl", "a", "true", "Show all url (suppress passing payload urls by default))")
	PayloadCmd.Flags().StringVarP(&payloadOpts.showAggrTimesStr, "showAggrTimes", "s", "true", "Show duration for underlying prowjobs for aggregated jobs")
	PayloadCmd.Flags().StringVarP(&payloadOpts.showSuccessStr, "showSuccess", "c", "false", "Show jobs even though they were successful (show only failed jobs by default)")
	PayloadCmd.Flags().StringVarP(&payloadOpts.dbMode, "dbMode", "d", "rcWebpage", "DB mode (rcWebpage (default), sippyDB, rcAPI)")
	PayloadCmd.Flags().StringVarP(&payloadOpts.printTestDetailStr, "printTestDetail", "t", "false", "Print test detail")
	PayloadCmd.Flags().StringVarP(&payloadOpts.showAggrJobDetailStr, "showAggrJobDetail", "j", "false", "Show aggregated job detail")
	return PayloadCmd
}

func (o *payloadOptsType) Run() {
	fmt.Println("Run called")
	fmt.Println("version:", o.version)
	fmt.Println("stream:", o.stream)
	fmt.Println("showAllUrl:", o.showAllUrl)
	fmt.Println("showAggrTimes:", o.showAggrTimes)
	fmt.Println("showSuccess:", o.showSuccess)
	fmt.Println("dbMode:", o.dbMode)
	fmt.Println("printTestDetail:", o.printTestDetail)
	fmt.Println("showAggrJobDetail:", o.showAggrJobDetail)

	defer fmt.Println("Finished listing the payloads")

	for i := 0; i < 12; i++ {
		fmt.Println()
	}
	fmt.Printf("Getting: %s %s\n", o.version, o.stream)

	payloadItems := payload_processing.GetPayloadItems(o.version, o.stream, payloadOpts.payload_getter)

	for _, payloadItem := range payloadItems {
		payload_processing.ProcessPayloadItem(payloadItem, o.showAllUrl, o.showAggrTimes, o.showSuccess, o.printTestDetail, o.showAggrJobDetail)
	}
}
