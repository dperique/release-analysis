package job_analysis

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/dperique/release-analysis/payload_processing"
	"github.com/spf13/cobra"
)

type analysisOptsType struct {
	url        string
	addDetails bool
}

var analysisOpts analysisOptsType

// Create the analysis command
var AnalysisCmd = &cobra.Command{
	Use:   "analysis aUrl",
	Short: "Analyze a particular payload or prow job",
	Long:  `View analysis of a payload url or prow job (add more details)...`,
	Args:  cobra.MinimumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("analysis called")
		analysisOpts.url = args[0]
		analysisOpts.Run()
	},
}

func NewAnalysisCmd() *cobra.Command {
	AnalysisCmd.Flags().BoolVarP(&analysisOpts.addDetails, "add_details", "d", false, "Payload url")
	return AnalysisCmd
}

func (a *analysisOptsType) Run() {
	fmt.Println("Run called")
	fmt.Println("url:", a.url)
	fmt.Println("addDetails:", a.addDetails)

	shortNamesMap := map[string]string{
		"aws-sdn-serial":         "aws-sdn-serial",
		"aws-sdn-upgrade":        "aws-sdn-upgrade",
		"e2e-aws-sdn-upgrade":    "aws-sdn-upgrade",
		"e2e-aws-ovn-upgrade":    "aws-ovn-upgrade",
		"e2e-aws-sdn-serial":     "aws-sdn-serial",
		"e2e-metal-ipi-ovn-ipv6": "metal-ipi-sdn-bm",
		"e2e-metal-ipi-sdn-bm":   "metal-ipi-sdn-bm",
		"e2e-metal-ipi-sdn":      "metal-ipi-sdn",
		"e2e-gcp-sdn":            "gcp-sdn",
		//"e2e-alibaba-ovn":               "alibaba-ovn", not supported
		"aggregated-azure-ovn-upgrade":  "azure-ovn-upgrade",
		"aggregated-gcp-ovn-rt-upgrade": "gcp-ovn-rt-upgrade",
		"aggregated-aws-sdn-upgrade":    "aws-sdn-upgrade",
		"aggregated-aws-ovn-upgrade":    "aws-ovn-upgrade",
		"aggregated-azure-sdn-upgrade":  "azure-sdn-upgrade",
		"aggregated-gcp-ovn-upgrade":    "gcp-ovn-upgrade",
	}

	// figure out what mode we are in depending on the url
	mode := "plain"
	if strings.Contains(a.url, "aggregated") {
		mode = "aggr"
	} else if strings.Contains(a.url, "releasestream") {
		mode = "payload"
	}
	if mode == "aggr" {

		fmt.Println("Aggregation job")

		// We are in pure aggregated job mode so ignore all the other args.
		aggrJobUrl := a.url

		// Extract the aggregated job name
		//re := regexp.MustCompile(`logs/(.*?)-4.(14|13)`)
		// Branching TODO: We have to add another version for every release
		re := regexp.MustCompile(`logs/(.*?)-4.(16|15|14|13).*?/(\d+)$`)

		match := re.FindStringSubmatch(aggrJobUrl)
		var aggrJobName string
		var aggrJobId string
		if len(match) > 2 {
			aggrJobName = match[1]
			aggrJobId = match[3]
		} else {
			fmt.Println("No idea what the aggregated job name is")
			os.Exit(1)
		}

		if shortName, ok := shortNamesMap[aggrJobName]; ok {
			payload_processing.PrintAggrSummaryTests(aggrJobUrl, true, true, a.addDetails, shortName)

			aggrJobUrlList, err := payload_processing.GetJobRunUrls(aggrJobUrl)
			if err != nil {
				fmt.Println(err)
			}
			// Put the aggregated job as the first url for convenience
			totalJobUrlList := append([]string{aggrJobUrl}, aggrJobUrlList...)
			fmt.Printf("\"aggr-%s-%s\": [\n", shortName, aggrJobId)
			len := len(totalJobUrlList)
			var comma string = ","
			for i, url := range totalJobUrlList {
				if i == (len - 1) {
					comma = ""
				}
				fmt.Printf("   \"%s\"%s\n", url, comma)
			}
			fmt.Println("],")
		} else {
			fmt.Println("Unable to determine short name for aggr job (needed to get a unit tests)")
			os.Exit(1)
		}
	}
	if mode == "plain" {

		fmt.Println("Plain job")

		plainJobUrl := a.url

		// Extract the job name
		re := regexp.MustCompile(`-4.(14|13)-(.*?)/\d+$`)
		match := re.FindStringSubmatch(plainJobUrl)
		var plainJobName string
		if len(match) > 1 {
			plainJobName = match[2]
		} else {
			fmt.Println("No idea what the plain job name is")
			os.Exit(1)
		}
		fmt.Println(plainJobName)
		if shortName, ok := shortNamesMap[plainJobName]; ok {
			payload_processing.PrintPlainSummaryTests(plainJobUrl, shortName, true, a.addDetails, "")
		} else {
			fmt.Println("Unable to determine short name for plain job (needed to get a unit tests)")
			os.Exit(1)
		}
		//https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.14-e2e-aws-ovn-upgrade/1649404378685116416
		// periodic-ci-openshift-release-master-ci-4.14-e2e-aws-sdn-serial
		// periodic-ci-openshift-release-master-ci-4.14-e2e-aws-ovn-upgrade
		// periodic-ci-openshift-release-master-nightly-4.14-e2e-metal-ipi-ovn-ipv6
		//   e2e-metal-ipi-ovn-ipv6/baremetalds-e2e-test/
		// periodic-ci-openshift-release-master-nightly-4.14-e2e-metal-ipi-sdn-bm
		//   e2e-metal-ipi-sdn-bm/baremetalds-e2e-test/
	}
	if mode == "payload" {
		fmt.Println("Payload item")
		payloadItem := payload_processing.ReleasePayload{
			ReleaseURL: a.url,
		}
		payload_processing.ProcessPayloadItem(payloadItem, true, true, false, true, true)
	}
	os.Exit(0)
}
