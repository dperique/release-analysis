package payload_processing

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	goutils "github.com/dperique/goutils"
)

const (
	red         = "\033[1;31m"
	green       = "\033[1;32m"
	purple      = "\033[1;34m"
	cyan        = "\033[36m"
	orange      = "\033[38;5;208m"
	colorNone   = "\033[0m"
	acceptedStr = "Accepted"
	rejectedStr = "Rejected"
	pendingStr  = "Pending"

	// GCS message shown when an artifact is not available (the page exists but this text shows)
	NOT_SERVING   = "The application is currently not serving requests at this endpoint. It may not have been started or is still starting"
	maxChar       = 175
	MAX_JOBS      = 10
	MAX_TESTS     = 20
	BODY_TIMEOUT  = 5
	JUNIT_TIMEOUT = 50

	// the release controller main page
	releaseUrlPrefix = "https://amd64.ocp.releases.ci.openshift.org/"
)

var (
	errDownloadTookTooLong = errors.New("download took too long")
	regexTitle             = regexp.MustCompile(`\<.*title\>(.*)\<\/title\>`)
)

// jobSummaryLineRegex is used to extract info about a specific job run in a job-run-summary.html file
var jobSummaryLineRegex = regexp.MustCompile(`\<li\>\<a target="_blank" href="(.*)"\>.*\</a\> build[0-9]+ (failure|success) after (.*)`)

// getJobStr takes the parts of a Job and returns a nicely formatted string that can be used to
// show a summary of it (this includes the last part of the Url, build farm server, time, and graph).
func getJobStr(jobUrl, jobStatusFull, jobTime string) string {
	tailOfJobUrl := strings.Split(jobUrl, "/")
	jobStatus := jobStatusFull[0:4]
	t, _ := time.ParseDuration(jobTime)

	// Create a string of asterisks proportional to the number of seconds (scaled down by 1000)
	// so we can visually ascertain relative job time.
	scaleSeconds := int(t.Seconds() / 1000.0)
	var stars string
	for j := 0; j < scaleSeconds; j++ {
		stars = stars + "*"
	}
	buildLogUrl := strings.Replace(jobUrl, "https://prow.ci.openshift.org/view/gs", "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs", 1)
	prowJobJsonUrl := buildLogUrl + "/prowjob.json"
	buildFarmNum := getBuildFarmServer(prowJobJsonUrl)
	return fmt.Sprintf("    %s %s %4s %8s %s", tailOfJobUrl[len(tailOfJobUrl)-1], buildFarmNum, jobStatus, jobTime, stars)
}

// getBuildFarmServer takes a string which is the Url for the prowjob.json file from a prow job.  In this
// file, there's a string like `"cluster": "build05"`, which has the build farm server name.  This
// function finds that and returns it.
func getBuildFarmServer(prowJobJsonUrl string) string {
	body, err := getBodyTimeout(prowJobJsonUrl, BODY_TIMEOUT)
	if err != nil {
		fmt.Println("third part")
		if err == errDownloadTookTooLong {
			fmt.Println("Download problem 6")
			goutils.CheckErrFatal(err)
		} else {
			fmt.Println("Other third error")
			goutils.CheckErrFatal(err)
		}
		return "build??"
	}

	clusterRegex := regexp.MustCompile(`"cluster": "(.*)",`)
	for _, line := range strings.Split(string(body), "\n") {
		m := clusterRegex.FindStringSubmatch(line)
		if len(m) > 1 {
			return m[1]
		}
	}
	return "build??"
}

// getBodyTimeout takes a url, and returns the body (i.e., contents).
// This is the same thing you get when you do curl -sk url.
// If timeout is exceeded during the ReadAll() call, return nil.
// Any error is fatal except timeout.
func getBodyTimeout(url string, timeout int) ([]byte, error) {
	respCh := make(chan *http.Response, 1)
	errCh := make(chan error, 1)
	go func() {
		resp, err := http.Get(url)
		goutils.CheckErrFatal(err)
		respCh <- resp
		errCh <- err
	}()
	var resp *http.Response
	var err error
	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		fmt.Println("Download problem doing http.Get()")
		return nil, errDownloadTookTooLong
	case resp = <-respCh:
		err = <-errCh
		goutils.CheckErrFatal(err)
	}
	byteCh := make(chan []byte, 1)
	go func() {
		defer resp.Body.Close()
		bytes, err := io.ReadAll(resp.Body)
		goutils.CheckErrFatal(err)
		byteCh <- bytes
		errCh <- err
	}()
	select {
	case <-time.After(time.Duration(timeout) * time.Second):
		fmt.Println("Download problem doing io.ReadAll()")
		return nil, errDownloadTookTooLong
	case result := <-byteCh:
		err := <-errCh
		goutils.CheckErrFatal(err)
		return result, err
	}
}

// getSummaryPrefix takes an aggregated job and gets the artifacts directory
// prefix that will lead to its two summary files.
func getSummaryPrefix(aggrJobUrl string) string {
	// Depending on the gcs bucket name, we'll have to use a different pattern.
	var pattern string
	if strings.Contains(aggrJobUrl, "test-platform-results") {
		pattern = `https://prow.ci.openshift.org/view/gs/test-platform-results/logs/`
	} else {
		pattern = `https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/`
	}

	return strings.Replace(aggrJobUrl, pattern, "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/origin-ci-test/logs/", 1)
}

// getSummaryUrl takes an aggregated job and returns the aggregation-testrun-summary.html
// to be used to extract the test failures.
func getSummaryUrl(aggrJobUrl string) string {
	aggrSummaryPrefix := getSummaryPrefix(aggrJobUrl)
	aggrSummaryPostfix := "artifacts/release-analysis-aggregator/openshift-release-analysis-aggregator/artifacts/release-analysis-aggregator/aggregation-testrun-summary.html"
	return fmt.Sprintf("%s/%s", aggrSummaryPrefix, aggrSummaryPostfix)
}

// getJobSummaryUrl takes an aggregated job and returns the job-run-summary.html
// to be used to extract job run times (and job run urls).
func getJobSummaryUrl(aggrJobUrl string) string {
	aggrSummaryPrefix := getSummaryPrefix(aggrJobUrl)
	aggrJobSummaryPostfix := "artifacts/release-analysis-aggregator/openshift-release-analysis-aggregator/artifacts/release-analysis-aggregator/job-run-summary.html"
	return fmt.Sprintf("%s/%s", aggrSummaryPrefix, aggrJobSummaryPostfix)
}

// createSortedDurations takes an output string of the form "[ jobId=7s jobId=9s ... ]" and
// transform it into a sorted list of durations (without the s)
func createSortedDurations(output string) string {
	durationList := []int{}
	for _, durationPair := range strings.Split(output, " ") {
		durationStr := strings.Split(durationPair, "=")[1]
		// Chop the trailing "s" and convert to int
		tmp := strings.Replace(durationStr, "s", "", -1)
		f, err := strconv.ParseFloat(tmp, 32)
		if err != nil {
			fmt.Println("Error parsing float: ", tmp)
			f = 99999.0
		}
		durationInt := int(math.Round(f))
		durationList = append(durationList, durationInt)
	}
	sort.Ints(durationList)
	output = ""
	sep := ""
	for _, durationInt := range durationList {
		output += sep + fmt.Sprintf("%d", durationInt)
		sep = ", "
	}
	return output
}

// processJunit takes a gcs bucket url (junit subdir), downloads files that match the pattern,
// to /tmp/jobId.  Specifically put these somewhere on the disk so you can look at them manually for
// debugging.  When things work well, just keep the contents in memory (and notice memory size
// increase).
// returns a slice of those the filename for those files in /tmp/jobId
// TODO refactor with the gcs_node_download program
func processJunit(url, pattern string) []string {

	// Extract the job id from the URL (as it is unique).
	re := regexp.MustCompile(`\d{19}`)
	matches := re.FindStringSubmatch(url)

	if len(matches) == 0 {
		fmt.Println("Unable to extract job id, falling back to /tmp")
	}

	jobId := matches[0]

	// Get contents of the junit subdir.
	//n := time.Now()
	body, err := getBodyTimeout(url, BODY_TIMEOUT)
	//fmt.Printf("time to download junit subdir contents: %s\n", time.Since(n))
	if err != nil {
		if err == errDownloadTookTooLong {
			fmt.Println("Download problem 8")
			goutils.CheckErrFatal(err)
		}
	}
	fileList := strings.Split(string(body), "\n")

	regExp1 := regexp.MustCompile(`^.*<a href=\"`)
	regExp2 := regexp.MustCompile(`\"><.*$`)

	filePatternRegex := regexp.MustCompile(pattern)
	retVal := []string{}
	for _, i := range fileList {
		if strings.Contains(i, "gcs/origin-ci-test") {
			line := regExp1.ReplaceAllString(i, "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com")
			line = regExp2.ReplaceAllString(line, "")
			lineParts := strings.Split(line, "/")
			tmpDirPath := filepath.Join("/tmp", jobId)
			fileName := filepath.Join(tmpDirPath, lineParts[len(lineParts)-1])
			//fmt.Println("Debug: ", fileName)
			//fileName := "/tmp/" + jobId + "/" + lineParts[len(lineParts)-1]

			if !filePatternRegex.MatchString(fileName) {
				//fmt.Printf("match check: %s %s\n", pattern, fileName)
				continue
			}
			//fmt.Printf("  Getting: %s\n", fileName)
			err := os.MkdirAll(tmpDirPath, 0755)
			if err != nil {
				fmt.Printf("Failed to create directory %s\n", tmpDirPath)
				goutils.CheckErrFatal(err)
			}
			outputFile, err := os.Create(fileName)
			if err != nil {
				goutils.CheckErrFatal(err)
			}

			// Get the file contents.
			// Note the timeout of 50 seconds; this is because those junit.xml file are
			// sometimes in the 10M and 20M range.  GCS is probably throttling the speed
			// at which we can download.
			//n = time.Now()
			body, err = getBodyTimeout(line, JUNIT_TIMEOUT)
			//fmt.Printf("time to download junit file: %s\n", time.Since(n))
			if err != nil {
				if err == errDownloadTookTooLong {
					fmt.Println("Download problem 9")
					goutils.CheckErrFatal(err)
				}
			}

			// Write the contents to file.
			_, err = outputFile.Write(body)
			goutils.CheckErrFatal(err)
			_ = outputFile.Close()
			retVal = append(retVal, fileName)
		}
	}
	//fmt.Println("Done downloading")
	// Include the directory for later deletion
	retVal = append(retVal, jobId)
	return retVal
}

// printPayloadTitles prints out a payload title containing its status, time and url (for failed payloads)
func printPayloadTitles(showAllUrl bool, title string, payloadStatus string, payloadItem ReleasePayload) {
	var color string
	var url string
	switch payloadStatus {
	case rejectedStr:
		color = red
		url = payloadItem.ReleaseURL
	case acceptedStr:
		color = green
		url = ""
		if showAllUrl {
			url = payloadItem.ReleaseURL
		}
	case pendingStr, "Ready", "":
		color = cyan
		url = ""
		if showAllUrl {
			url = payloadItem.ReleaseURL
		}
	default:
		fmt.Println("Unknown payloadStatus:", payloadStatus, " should be one of:", acceptedStr, rejectedStr)
	}
	if payloadItem.forced {
		payloadStatus += "(f)"
	}

	fmt.Println()
	fmt.Println("===============================================================================================================================================================================")
	fmt.Println()
	fmt.Printf("%s%s  %s %s %11s %16s   %s\n", color, title, payloadStatus, colorNone, payloadItem.timeStr, payloadItem.timeDetailStr, url)
}

// processPayloadItem takes a payload item (containing an URL for the release) and scrapes the page
// to determine the pass/fail and aggregate job info and prints it.
//
// showAllUrl, showAggrTimes, showSuccess are values of the parameters passed into the main function.
func ProcessPayloadItem(payloadItem ReleasePayload, showAllUrl, showAggrTimes, showSuccess, printTestDetail, showAggrJobDetail bool) {
	body, err := getBodyTimeout(payloadItem.ReleaseURL, BODY_TIMEOUT*10)
	if err != nil {
		if err == errDownloadTookTooLong {
			fmt.Println("Download problem 7")
			goutils.CheckErrFatal(err)
		}
	}
	tlines := regexTitle.FindStringSubmatch(string(body))
	var title string
	if len(tlines) > 1 {
		title = tlines[1][8:]
	} else {
		title = "No release"
	}

	// The text below "Blocking jobs" is the Informing jobs.
	tmp := strings.Split(string(body), `Blocking jobs`)
	if len(tmp) < 2 {
		// If this happens, the payload webpage was most likely aged out (and deleted).
		fmt.Println()
		fmt.Println("===============================================================================================================================================================================")

		titleParts := strings.Split(payloadItem.ReleaseURL, "/")
		title = titleParts[len(titleParts)-1]
		displayedPhase := payloadItem.phase
		if payloadItem.forced {
			displayedPhase += "(f)"
		}
		fmt.Printf("%s %s, %s\n", title, displayedPhase, payloadItem.ReleaseURL)
		fmt.Println("  ", strings.Split(string(body), "\n")[0])

		// Realize that you can still get the urls for the blocking jobs via this:
		// sippy_openshift=> select url from release_job_runs join release_tags on release_tags.id = release_job_runs.release_tag_id where release_tags.release_tag='4.14.0-0.ci-2023-03-08-230640';
		//-------------------------------------------------------------------------------------------------------------------------------------------------------------------------
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.14-e2e-aws-sdn-serial/1633606785149440000
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/aggregated-aws-ovn-upgrade-4.14-micro-release-openshift-release-analysis-aggregator/1633606765071306752
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/aggregated-aws-ovn-upgrade-4.14-minor-release-openshift-release-analysis-aggregator/1633606775087304704
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/aggregated-azure-sdn-upgrade-4.14-minor-release-openshift-release-analysis-aggregator/1633606784306384896
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.14-e2e-gcp-sdn/1633606786013466624
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.14-upgrade-from-stable-4.13-e2e-aws-sdn-upgrade/1633606763443916800

		// sippy_openshift=> select release_job_runs.url, prow_job_runs.succeeded from release_job_runs
		// join release_tags on release_tags.id = release_job_runs.release_tag_id
		// join prow_job_runs on prow_job_runs.url = release_job_runs.url
		// where release_tags.release_tag='4.14.0-0.ci-2023-03-08-230640';
		//                                                                                   url                                                                                   | succeeded
		//-------------------------------------------------------------------------------------------------------------------------------------------------------------------------+-----------
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.14-e2e-gcp-sdn/1633606786013466624                                  | t
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.14-upgrade-from-stable-4.13-e2e-aws-sdn-upgrade/1633606763443916800 | f
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/aggregated-aws-ovn-upgrade-4.14-micro-release-openshift-release-analysis-aggregator/1633606765071306752       | f
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/aggregated-aws-ovn-upgrade-4.14-minor-release-openshift-release-analysis-aggregator/1633606775087304704       | f
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/aggregated-azure-sdn-upgrade-4.14-minor-release-openshift-release-analysis-aggregator/1633606784306384896     | t
		// https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/periodic-ci-openshift-release-master-ci-4.14-e2e-aws-sdn-serial/1633606785149440000                           | t

		return
	}
	blines := tmp[1]

	// The text before "Informing jobs" is the Blocking jobs (the part we care about).
	ilines := strings.Split(blines, `Informing jobs`)[0]

	// Operate on the blocking jobs one at a time.
	// These will be a list of <a class... href=http... Pending|Succeeded|Failed</a>.
	blockingJobsList := strings.Split(ilines, `<li>`)

	var payloadStatus string

	payloadStatus = acceptedStr

	succeededOrFailed := regexp.MustCompile(`href="(.*)"\>(.*) (Pending.*|Succeeded.*|Failed.*)</a>.*`)

	// The loop is done twice -- once to calculate if the payload was Accepted or Rejected,
	// and once to print it out.
	for _, line := range blockingJobsList {
		list := succeededOrFailed.FindStringSubmatch(line)
		if len(list) < 2 {
			// Skip any other html element that doesn't match.
			continue
		}
		status := list[3]
		if status == "Failed" {
			payloadStatus = rejectedStr
		}
		if status == "Pending" {
			payloadStatus = pendingStr
		}
	}

	fmt.Println(payloadStatus)
	// We already figured out the payload status earlier; but let's ensure they match.
	if payloadStatus != payloadItem.phase {
		goutils.CheckErrFatal(err)

		// This happens when we force reject a payload and the jobs are still running (and in
		// Pending state) so we wrongly conclude the payload is pending when it never finished,
		// was force rejected and will never finish.
		// Therefore, jobs that show up as Rejected and have no info after it, are probably
		// force rejected.
		payloadStatus = payloadItem.phase
	}

	// Now that we know the payload status, print the payload title and status.
	printPayloadTitles(showAllUrl, title, payloadStatus, payloadItem)

	for _, line := range blockingJobsList {
		list := succeededOrFailed.FindStringSubmatch(line)
		if len(list) < 3 {
			continue
		}
		payloadJobShortName := strings.Trim(list[2], " ")

		// Get the status and print only the ones that failed.
		status := list[3]

		if status == "Failed" || showSuccess {
			if status == "Failed" {
				fmt.Println(" ", payloadJobShortName, red, status, colorNone)
			}
			if status == "Succeeded" {
				fmt.Println(" ", payloadJobShortName, green, status, colorNone)
			}

			if strings.HasPrefix(payloadJobShortName, "aggregated") {
				// This looks like an aggregated job so extract the aggregated job url.
				aggrJobUrl := list[1]

				// Goto the aggregated job and print out the failing tests
				PrintAggrSummaryTests(aggrJobUrl, showAggrTimes, printTestDetail, showAggrJobDetail, payloadJobShortName)
			} else {
				plainJobUrl := list[1]
				output := PrintPlainSummaryTests(plainJobUrl, payloadJobShortName, true, printTestDetail, "")
				for _, line := range output {
					fmt.Println(line)
				}
			}
		}
	}
}

// printPlainSummaryTests takes the URL of a prow job and a short name and returns output
// lines that represent the name of the tests that failed.
// payloadJobShortName: used to determine where the junit xml files reside.
// displayUrl: allows us to not display the url esp. when called for aggregated job processing
// printTestDetail: enables printing test failure output (it gets verbose so suppress most of the time)
// extraSpace: depending on what calls this function, we may need more space to make the output look clean
// If we have trouble parsing the xml file (e.g., bad character present), we return an error string so that
// when it's output, we can see something went wrong.
func PrintPlainSummaryTests(plainJobUrl, payloadJobShortName string, displayUrl bool, printTestDetail bool, extraSpace string) []string {

	type Property struct {
		Name  string `xml:"name,attr"`
		Value string `xml:"value,attr"`
	}

	type Failure struct {
		Message string `xml:"message,attr"`
		Content string `xml:",chardata"`
	}

	type Testcase struct {
		Name      string   `xml:"name,attr"`
		Time      string   `xml:"time,attr"`
		Failure   *Failure `xml:"failure"`
		SystemOut string   `xml:"system-out"`
	}

	type Testsuite struct {
		XMLName   xml.Name   `xml:"testsuite"`
		Name      string     `xml:"name,attr"`
		Tests     string     `xml:"tests,attr"`
		Skipped   string     `xml:"skipped,attr"`
		Failures  string     `xml:"failures,attr"`
		Time      string     `xml:"time,attr"`
		Property  []Property `xml:"property"`
		Testcases []Testcase `xml:"testcase"`
	}

	type TestSuites struct {
		XMLName   xml.Name    `xml:"testsuites"`
		TestSuite []Testsuite `xml:"testsuite"`
	}

	if strings.Contains(payloadJobShortName, "4.12") {
		// Hack, maintenance item for every GA'ed release: when we convert to aggregated
		// jobs on a GA'ed release to non-aggregated, the short name changes to
		// "aws-ovn-upgrade-4.12-micro" (i.e., it will no longer have the string
		// "aggregated" in it).  This tool was created around 4.13 time so we only
		// have to deal with this in 4.12.
		payloadJobShortName = strings.Split(payloadJobShortName, "-4.12")[0]
	}
	if displayUrl {
		fmt.Println("   ", plainJobUrl)
	}

	// Form the path for where to find the junit xml file
	junitXmlUrl := strings.Replace(plainJobUrl, "https://prow.ci.openshift.org/view/gs", "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs", 1)
	fallbackXml := junitXmlUrl + "/artifacts"
	usingFallback := false

	if strings.HasPrefix(payloadJobShortName, "aggregated-") {
		// Strip off the "aggregated-" and trim off the "4.x" to arrive at a proper shortname.
		payloadJobShortName = strings.ReplaceAll(payloadJobShortName, "aggregated-", "")
		payloadJobShortName = strings.Split(payloadJobShortName, "-4.")[0]
	}

	// Depending on the testName, the location of the junit xml file varies.
	switch payloadJobShortName {
	case "metal-ipi-sdn":
	case "metal-ipi-ovn-ipv6":
	case "e2e-metal-ipi-sdn":
		junitXmlUrl += "/artifacts"
		usingFallback = true
	case "aws-ovn-fips":
		junitXmlUrl += "/artifacts/e2e-aws-ovn-fips/openshift-e2e-test/artifacts/junit"
	case "install-analysis-all":
		// This one is similar to an aggregated job and has junit files scrattered about; implement it someday.
		junitXmlUrl += "Do this someday"
	default:
		// for our tests that use openshift-e2e-test (the ones we care about on blocking jobs), this
		// will be the location of the junit xml.
		junitXmlUrl += fmt.Sprintf("/artifacts/e2e-%s/openshift-e2e-test/artifacts/junit", payloadJobShortName)
	}

	// Look for the junit file in the directory we found.
	xmlFiles := processJunit(junitXmlUrl, "xml")

	// Track what files to close and cleanup
	cleanupXmlFiles := xmlFiles
	fileList := []*os.File{}

	if len(xmlFiles) < 1 {
		//fmt.Println(t)
		//fmt.Println("    NO XML FILES FOUND; trying fallback")
		xmlFiles = processJunit(fallbackXml, "xml")
		cleanupXmlFiles = xmlFiles
		usingFallback = true
	}
	//fmt.Println(xmlFiles)

	failedTestOutput := []string{}
	for _, xmlFile := range xmlFiles {
		if !strings.Contains(xmlFile, ".xml") {
			continue
		}
		file, err := os.Open(xmlFile)
		if err != nil {
			goutils.CheckErrFatal(err)
			return []string{fmt.Sprintf("Unable to open xml file: %s", xmlFile)}
		}
		fileList = append(fileList, file)
		var testsuite Testsuite
		if usingFallback {
			// metal and fallback mode have TestSuites ; we get the first Testsuite.
			var testSuites TestSuites
			if err := xml.NewDecoder(file).Decode(&testSuites); err != nil {
				goutils.CheckErrFatal(err)
				return []string{"Could not parse xml file 1"}
			}
			testsuite = testSuites.TestSuite[0]
		} else {
			if err := xml.NewDecoder(file).Decode(&testsuite); err != nil {
				goutils.CheckErrFatal(err)
				return []string{"Could not parse xml file 2"}
			}
		}
		//fmt.Println("Testsuite:")
		//fmt.Printf("  Name: %s\n", testsuite.Name)
		//fmt.Printf("  Tests: %s\n", testsuite.Tests)
		//fmt.Printf("  Skipped: %s\n", testsuite.Skipped)
		//fmt.Printf("  Failures: %s\n", testsuite.Failures)
		//fmt.Printf("  Time: %s\n", testsuite.Time)

		// Count how many of each test case there is; fail is len of 1,
		// flake is len > 1.
		m := make(map[string][]Testcase, len(testsuite.Testcases))
		for _, testcase := range testsuite.Testcases {
			m[testcase.Name] = append(m[testcase.Name], testcase)
		}
		for _, testcase := range testsuite.Testcases {
			if testcase.Failure != nil {
				isFailure := false
				if len(m[testcase.Name]) == 1 {
					// One case and failure output exists, implies failure.
					isFailure = true
				}
				if len(m[testcase.Name]) == 2 {
					// Two cases and failure output exists, might be a flake.
					gotPass := false
					for _, t := range m[testcase.Name] {
						if t.Failure == nil {
							gotPass = true
						}
					}
					// If one of them was a pass, this is a flake
					if !gotPass {
						isFailure = true
					}
				}
				if isFailure {

					failureColor := purple
					// NOTE: You have to do this check twice: once for aggregated job and once for plain jobs
					if strings.Contains(testcase.Name, "disruption") || strings.Contains(testcase.Name, "Application behind service load balancer with PDB remains available using new connections") {
						// Color disruption in orange
						failureColor = orange
					}

					// The failure of these tests doesn't seem to contribute to the analysis so skip them
					// to keep the output useful; in the future, we may show them.
					if strings.Contains(testcase.Name, "observers-resource-watch container test") ||
						strings.Contains(testcase.Name, "openshift-e2e-test container test") ||
						strings.Contains(testcase.Name, "multi-stage test test phase") {
						continue
					}
					failedTestOutput = append(failedTestOutput, fmt.Sprintf("    %s%sFailed: %s%s\n", extraSpace, failureColor, testcase.Name, colorNone))

					if printTestDetail && strings.Contains(testcase.Name, "disruption") {
						if len(testcase.Failure.Content) > 0 {
							failedTestOutput = append(failedTestOutput, fmt.Sprintln("     ", extraSpace, strings.Split(testcase.Failure.Content, "\n")[0]))
						} else if len(testcase.Failure.Message) > 0 {
							failedTestOutput = append(failedTestOutput, fmt.Sprintln("     ", extraSpace, strings.Split(testcase.Failure.Message, "\n")[0]))
						}
					}
				}
			}
		}
	}
	for i, cleanupFile := range cleanupXmlFiles {
		if !strings.Contains(cleanupFile, ".xml") {
			continue
		}
		//fmt.Println("Closing ", cleanupFile)
		fileList[i].Close()
		err := os.Remove(cleanupFile)
		//fmt.Println("Done Closing ", cleanupFile)
		if err != nil {
			fmt.Println("Error deleting file:", err)
		}
	}
	jobIdDirPath := filepath.Join("/tmp", cleanupXmlFiles[len(cleanupXmlFiles)-1])
	err := os.RemoveAll(jobIdDirPath)
	//fmt.Println("Remove dir: ", jobIdDirPath)
	if err != nil {
		fmt.Printf("Failed to remove directory %s: %s\n", jobIdDirPath, err)
	}
	return failedTestOutput
}

// getJobRunUrls takes an aggregated job and returns a list of the job run urls.
func GetJobRunUrls(aggrJobUrl string) ([]string, error) {
	aggrJobSummaryUrl := getJobSummaryUrl(aggrJobUrl)

	// The code below is identical to the one from printAggrSummaryTests
	// except we don't need to go into each of the jobUrls.  This is a
	// refactor opportunity so we keep the code looking almost identical.
	// Get the html file for the aggregated job summary.
	body, err := getBodyTimeout(aggrJobSummaryUrl, BODY_TIMEOUT)
	if err != nil {
		fmt.Println("Download problem in getJobRunUrls")
		if err == errDownloadTookTooLong {
			fmt.Println("Download took long for getJobRunUrls")
			goutils.CheckErrFatal(err)
		} else {
			fmt.Println("Other error in download for getJobRunUrls")
			goutils.CheckErrFatal(err)
		}
		return []string{}, fmt.Errorf("error getting job-run-sumary.html for %s", aggrJobUrl)
	}

	lines := strings.Split(string(body), "\n")

	// There will always be MAX_JOBS jobs so we count them, and, once found, we don't need to continue.
	// The jobs are processed as go routines because it's a slow process to get the prowjob.json
	// artifacts and process them and we need to do it MAX_JOBS times.
	actualJobCount := 0

	retVal := []string{}

	// Ensure we got all MAX_JOB jobs
	foundAllJobs := false
	for i := 0; i < len(lines); i++ {
		m := jobSummaryLineRegex.FindStringSubmatch(lines[i])
		if len(m) > 1 {
			jobUrl := m[1]
			retVal = append(retVal, jobUrl)
			actualJobCount++
		}
		if actualJobCount == MAX_JOBS {
			// All jobs found so bail.
			foundAllJobs = true
			break
		}
	}
	if !foundAllJobs {
		fmt.Printf("    %sWarning: Got %d of %d jobs%s\n", red, actualJobCount, MAX_JOBS, colorNone)
	}
	return retVal, nil
}

// printAggrSummaryTests prints out the failure summary for an aggregated job so you don't have to
// click through to analyze its results.
// aggrJobUrl: the url for the aggregated job
// showAggrTimes: allows us to print how long each underlying job took (including an asterisk graph)
// printTestDetail: allows us to print out test failure output (it gets verbose so suppress if needed)
// payloadJobShortName: short name used by processJunit function to determine location of junit.xml files
func PrintAggrSummaryTests(aggrJobUrl string, showAggrTimes bool, printTestDetail bool, showAggrJobDetail bool, payloadJobShortName string) {

	// Get the aggregation prefix summary html file
	// aggrSummaryPrefix := strings.Replace(aggrJobUrl, "https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/", "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/origin-ci-test/logs/", 1)
	// aggrSummaryPostfix := "artifacts/release-analysis-aggregator/openshift-release-analysis-aggregator/artifacts/release-analysis-aggregator/aggregation-testrun-summary.html"
	// aggrJobSummaryPostfix := "artifacts/release-analysis-aggregator/openshift-release-analysis-aggregator/artifacts/release-analysis-aggregator/job-run-summary.html"
	// aggrSummaryUrl := fmt.Sprintf("%s/%s", aggrSummaryPrefix, aggrSummaryPostfix)
	// aggrJobSummaryUrl := fmt.Sprintf("%s/%s", aggrSummaryPrefix, aggrJobSummaryPostfix)
	aggrSummaryUrl := getSummaryUrl(aggrJobUrl)
	aggrJobSummaryUrl := getJobSummaryUrl(aggrJobUrl)
	fmt.Println("   ", aggrJobUrl)
	//fmt.Println("     ", aggrSummaryUrl)

	// Get the html file for the aggregated job summary.
	body, err := getBodyTimeout(aggrSummaryUrl, BODY_TIMEOUT)
	if err != nil {
		fmt.Println("first part")
		if err == errDownloadTookTooLong {
			fmt.Println("Download problem 4")
			goutils.CheckErrFatal(err)
		} else {
			fmt.Println("Other error")
			goutils.CheckErrFatal(err)
		}
		return
	}

	summaryPattern := regexp.MustCompile(`Passed (\d+) times, failed (\d+) times, skipped (\d+) times: we require at least one pass to consider it a success`)
	summaryPattern2 := regexp.MustCompile(`Passed (\d+) times, failed (\d+) times, skipped (\d+) times: we require at least (\d+) attempts to have a chance at success`)
	historicalSummaryPattern := regexp.MustCompile(`Failed: Passed (\d+) times, failed (\d+) times.  The historical pass rate is (\d+)%.  The required number of passes is (\d+).`)
	disruptionSummaryPattern := regexp.MustCompile(`Failed: Passed (\d+) times, failed (\d+) times.  \(.*requiredPasses=(\d+).*\)`)
	disruptionSummaryPattern2 := regexp.MustCompile(`Failed: Mean disruption of ([a-z-]+) is (\d+\.\d+) seconds is more than the failureThreshold`)
	disruptionSummaryPattern3 := regexp.MustCompile(`\((P[0-9]+=[0-9\.]+s).* failures=\[(.*)\]`)

	// Scrape the failed tests from the html file.
	lines := strings.Split(string(body), "\n")

	testsPrinted := 0
	// We have to track if there were actual failures or skips because even if the aggregation-testrun-summary.html
	// file doesn't exist, we still get a file and this makes it so that we need logic to determine if the file
	// actually got created vs. didn't get creatd and is a template file indicating missing file.
	foundFailures := false
	foundPassOrSkipped := false
	disruptionFailureCount := 0
	totalFailures := 0
	for i := 0; i < len(lines); i++ {
		maxTestIncr := 1
		//fmt.Printf("line => %s\n", lines[i])
		if strings.HasPrefix(lines[i], "Failed: ") {
			foundFailures = true
			failTestStr := strings.Replace(lines[i], "<b>", "", 1)
			failTestStr = strings.Replace(failTestStr, "</b>", "", 1)
			if len(failTestStr) > maxChar {
				failTestStr = failTestStr[:maxChar]
			}
			color := purple
			// NOTE: You have to do this check twice: once for aggregated job and once for plain jobs
			if strings.Contains(failTestStr, "disruption") || strings.Contains(failTestStr, "Application behind service load balancer with PDB remains available using new connections") {

				// Since disruption is being difficult lately, let's not count them for max tests.
				// This way, we can see if all failures are disruption related.
				// Set the color to orange.
				maxTestIncr = 0
				disruptionFailureCount++
				color = orange
			}
			fmt.Printf("    %s%s%s\n", color, failTestStr, colorNone)
			totalFailures++

			// The next line is the summary for this test.
			failStr := strings.Replace(lines[i+1], "<p>", "", 1)
			failStr = strings.Replace(failStr, "</p>", "", 1)

			// Summarize the pass/fail/historical line to be easy on the eyes.
			var passed, failed, skipped, historicalPassRate, requiredPasses int
			var m []string
			if m = summaryPattern.FindStringSubmatch(failStr); len(m) > 1 {
				passed, _ = strconv.Atoi(m[1])
				failed, _ = strconv.Atoi(m[2])
				skipped, _ = strconv.Atoi(m[3])
				failStr = fmt.Sprintf("pass=%d/fail=%d/skip=%d", passed, failed, skipped)

			} else if m = summaryPattern2.FindStringSubmatch(failStr); len(m) > 1 {
				passed, _ = strconv.Atoi(m[1])
				failed, _ = strconv.Atoi(m[2])
				skipped, _ = strconv.Atoi(m[3])
				requiredPasses, _ = strconv.Atoi(m[4])
				failStr = fmt.Sprintf("pass=%d/fail=%d/req=%d/skip=%d", passed, failed, requiredPasses, skipped)

			} else if m = historicalSummaryPattern.FindStringSubmatch(failStr); len(m) > 1 {
				passed, _ = strconv.Atoi(m[1])
				failed, _ = strconv.Atoi(m[2])
				historicalPassRate, _ = strconv.Atoi(m[3])
				requiredPasses, _ = strconv.Atoi(m[4])
				failStr = fmt.Sprintf("pass=%d/fail=%d/req=%d  historical=%d%%", passed, failed, requiredPasses, historicalPassRate)

			} else if m = disruptionSummaryPattern3.FindStringSubmatch(failStr); len(m) > 1 {
				// The output will show [ jobId=7s jobId=9s ... ]
				pNumber := m[1]
				output := m[2]
				failStr = fmt.Sprintf("pass=0/fail=10/req=? disruption, %s, %s", pNumber, createSortedDurations(output))

			} else if m = disruptionSummaryPattern.FindStringSubmatch(failStr); len(m) > 1 {
				passed, _ = strconv.Atoi(m[1])
				failed, _ = strconv.Atoi(m[2])
				requiredPasses, _ = strconv.Atoi(m[3])
				failStr = fmt.Sprintf("pass=%d/fail=%d/req=%d disruption", passed, failed, requiredPasses)
				fmt.Println("If you see the old disruption pattern getting matched, consider keeping it")
				fmt.Println("If it's significantly later than Mar 20, 2023, then consider removing this pattern")

			} else if m = disruptionSummaryPattern2.FindStringSubmatch(failStr); len(m) > 1 {
				deviation := m[2]
				failStr = fmt.Sprintf("pass=?/fail=?/req=? dev=%s disruption", deviation)

			} else {
				//fmt.Println("No match for summary line")
				//fmt.Println("We got: ", failStr)
				failStr = fmt.Sprintf("%s (?disruption)", failStr)

				// People who mess with the disruption output wack our regex so color these
				// bad lines as disruption.
				if strings.HasPrefix(failStr, "suite=[BackendDisruption") {
					disruptionFailureCount++
				}
				color = orange
			}

			fmt.Println("     ", failStr)
			if testsPrinted > MAX_TESTS {
				// If we already printed MAX_TESTS tests, we really need to just look at the prow job.
				// A summary greater than MAX_TESTS is just be too big for a human to want to look.
				fmt.Println("\n", " ", red, "THERE ARE MORE THAN", MAX_TESTS, " *********************************\n", colorNone)
				break
			}
			testsPrinted += maxTestIncr
			i++
		}
		if strings.HasPrefix(lines[i], "Skipped:") || strings.HasPrefix(lines[i], "Passed") {
			foundPassOrSkipped = true
		}
		if strings.Contains(lines[i], NOT_SERVING) {
			fmt.Println("    Aggregated job summary unavailable")
			//fmt.Println("   ", aggrSummaryUrl)
		}
	}
	if !foundFailures && !foundPassOrSkipped {
		// We didn't find any failures or passes/skips so most likely never got a genuine aggregation-testrun-summary.html so warn the user.
		fmt.Println(red, "   No failures found (aggregation-testrun-summary.html is probably missing)", colorNone)
	}

	if !showAggrTimes {
		return
	}

	if disruptionFailureCount > 0 {
		fmt.Println()
		fmt.Printf("    %s%s %d/%d%s\n", red, "Disruption failure count:", disruptionFailureCount, totalFailures, colorNone)
		fmt.Println()
	}

	// Print out the run times of each job (full complete runs ~3 hours)

	// Get the html file for the aggregated job summary.
	body, err = getBodyTimeout(aggrJobSummaryUrl, BODY_TIMEOUT)
	if err != nil {
		fmt.Println("second part")
		if err == errDownloadTookTooLong {
			fmt.Println("Download problem 5")
			goutils.CheckErrFatal(err)
		} else {
			fmt.Println("Other error")
			goutils.CheckErrFatal(err)
		}
		return
	}

	lines = strings.Split(string(body), "\n")

	// There will always be MAX_JOBS jobs so we count them, and, once found, we don't need to continue.
	// The jobs are processed as go routines because it's a slow process to get the prowjob.json
	// artifacts and process them and we need to do it MAX_JOBS times.
	type jobInfo struct {
		jobSummary string
		jobUrl     string // carry this along so we can print the results in the order they came
	}
	jobInfoCh := make(chan jobInfo, MAX_JOBS)
	actualJobCount := 0

	var wg sync.WaitGroup

	// Ensure we got all MAX_JOB jobs
	foundAllJobs := false
	for i := 0; i < len(lines); i++ {
		if i == 25 {
			fmt.Println()
		}
		m := jobSummaryLineRegex.FindStringSubmatch(lines[i])
		if len(m) > 1 {
			wg.Add(1)
			go func(jobUrl, jobStatusFull, jobTime string) {
				defer wg.Done()
				jobInfoCh <- jobInfo{
					jobSummary: getJobStr(jobUrl, jobStatusFull, jobTime),
					jobUrl:     jobUrl,
				}
			}(m[1], m[2], m[3])
			actualJobCount++
		}
		if actualJobCount == MAX_JOBS {
			// All jobs found so bail.
			foundAllJobs = true
			break
		}
	}
	if !foundAllJobs {
		fmt.Printf("    %sWarning: Got %d of %d jobs%s\n", red, actualJobCount, MAX_JOBS, colorNone)
	}

	// Wait until all of them are done and then close the channel; we need to do this
	// because this is a buffered channel being used as a queue where we will always
	// have MAX_JOBS jobs to process.
	wg.Wait()
	close(jobInfoCh)

	// When we parse out individual job results, it's slow becaues the junit.xml files and sometimes
	// big.  So, we launch a go routine for each and show the output lines as they come in; this way
	// the output can progress vs. always waiting for the slowest one and having the longest pause.
	jobOutputCh := make(chan []string, MAX_JOBS)
	counter := 0
	for jobInfoItem := range jobInfoCh {
		//fmt.Println("Launch: ", counter)
		counter++
		go func(jj jobInfo) {
			output := []string{
				jj.jobSummary,
				"\n",
			}

			if strings.Contains(jj.jobSummary, "fail") && showAggrJobDetail {
				// For jobs that failed, print out what tests failed.
				output = append(output, PrintPlainSummaryTests(jj.jobUrl, payloadJobShortName, false, printTestDetail, "  ")...)
			}
			jobOutputCh <- output
		}(jobInfoItem)
	}

	// We know exactly how many jobs there are (not always 10) so wait for this many.
	i := actualJobCount
	const waitSeconds = 60
	timeout := time.After(waitSeconds * time.Second)

	for i > 0 {
		select {
		case outputLines := <-jobOutputCh:
			i--
			for _, line := range outputLines {
				fmt.Printf("%s", line)
			}
		case <-timeout:
			fmt.Printf("Took greater than %ds to show job details; skipping ...\n", waitSeconds)
			i = 0
		}
	}
	close(jobOutputCh)
	fmt.Println()
}
