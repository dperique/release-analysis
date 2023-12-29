package payload_processing

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	goutils "github.com/dperique/goutils"
)

type dbModeType int

const (
	rcWebpage dbModeType = iota
	sippyDB
	rcAPI
)

type payloadGetter interface {
	// Given a release version and stream, return a list of payload items.
	getUrls(aVersion, aStream string) []ReleasePayload
}

// These are the three ways we can get the payload items.
type rcWebpagePayloadGetter struct{}
type sippyDBPayloadGetter struct{}
type rcAPIPayloadGetter struct{}

// getUrls returns a list of URLs (one for each payload) given a version and type (e.g., version=4.13, type=nightly)
// from the main release controller page.
// aVersion is like 4.12, 4.13, 4.14
// aStream is like nightly or ci
// TODO: this is a little wacky in that we "chop" parts to get to the part we want. Convert to getUrlsFromSippy.
func (g rcWebpagePayloadGetter) getUrls(aVersion, aStream string) []ReleasePayload {
	releaseStr := fmt.Sprintf("%s/#%s.0-0.%s", releaseUrlPrefix, aVersion, aStream)
	body, err := getBodyTimeout(releaseStr, BODY_TIMEOUT)
	var ret []ReleasePayload
	if err != nil {
		if err == errDownloadTookTooLong {
			fmt.Println("Download problem 3")
			goutils.CheckErrFatal(err)
		}
	}
	var currfStr string
	switch aStream {
	case "nightly":
		currfStr = fmt.Sprintf("This release contains OSBS official image builds of all code in release-%s (master) branches, and is updated after those builds are synced to quay.io.", aVersion)
	case "ci":
		currfStr = fmt.Sprintf("This release contains CI image builds of all code in release-%s (master) branches, and is updated each time someone merges.", aVersion)
	default:
		goutils.CheckErrFatal(fmt.Errorf("bad value for aType: %s", aStream))
	}
	t := strings.Split(string(body), currfStr)
	x := t[1]
	var prevVer string

	// Branching TODO: we have to add another version for every release
	switch aVersion {
	case "4.16":
		prevVer = "4.15"
	case "4.15":
		prevVer = "4.14"
	case "4.14":
		prevVer = "4.13"
	case "4.13":
		prevVer = "4.12"
	case "4.12":
		prevVer = "4.11"
	default:
		goutils.CheckErrFatal(fmt.Errorf("bad value for aVersion: %s", aVersion))
	}

	var prevfStr string
	var endChar int
	switch aStream {
	case "nightly":
		prevfStr = fmt.Sprintf("This release contains CI image builds of all code in release-%s (master) branches, and is updated each time someone merges.", prevVer)
		endChar = 89
	case "ci":
		prevfStr = fmt.Sprintf("This release contains OSBS official image builds of all code in release-%s (master) branches, and is updated after those builds are synced to quay.io.", prevVer)
		endChar = 79
	default:
		goutils.CheckErrFatal(fmt.Errorf("bad value for aType: %s", aStream))
	}

	u := strings.Split(x, prevfStr)
	v := u[0]
	regex1 := regexp.MustCompile(`<td class="text-monospace"><a class=".*" href="/`)
	regexAcceptOrReject := regexp.MustCompile(`>([^<]+)<`)
	regexPayloadTime := regexp.MustCompile(`<td title="(.*)">([^<]+)<`)

	// By time you get here, v is at <table id="4.13.0-0.nightly_table" class="table text-nowrap">
	// If you "grep '\(nightly\|ci\)_table' index.html ", you can see that there is a single table
	// with that name per release.  This means we can look for that when we parse instead of doing
	// the "chopping" method based on strings above and make the code above less hacky.
	lines := strings.Split(v, "\n")
	for i := 0; i < len(lines); i++ {
		line := lines[i]
		if regex1.MatchString(line) {
			versionStr := fmt.Sprintf("%s.0-0.%s", aVersion, aStream)
			if !strings.Contains(line, versionStr) {
				continue
			}
			str1 := regex1.ReplaceAllString(line, "")
			rel := fmt.Sprintf("%s%s", releaseUrlPrefix, str1[16:endChar])

			// Once we found the release url line, the next line contains the state (Accept or Reject).
			var payloadStatus string = "unknown state"
			if strings.Contains(lines[i+1], acceptedStr) || strings.Contains(lines[i+1], rejectedStr) || strings.Contains(lines[i+1], "Ready") {
				payloadStatus = regexAcceptOrReject.FindStringSubmatch(lines[i+1])[1]
			}

			// The following line contains the payload time.
			var payloadTime string = "unknown time"
			var payloadTimeDetail string = "unknown time"
			payloadTimeMatches := regexPayloadTime.FindStringSubmatch(lines[i+2])
			if len(payloadTimeMatches) > 2 {
				payloadTimeDetail = payloadTimeMatches[1][5:]
				payloadTime = payloadTimeMatches[2]
			}
			payloadItem := ReleasePayload{
				ReleaseURL:    rel,
				phase:         payloadStatus,
				timeStr:       payloadTime,
				timeDetailStr: payloadTimeDetail,
			}
			ret = append(ret, payloadItem)
			i = i + 2
		}
	}
	return ret
}

// getUrls fetches the urls from the sippy database.
// The results will be delayed because sippy's fetchdata runs hourly to sync wiht latest test runs.
// This is a cleaner way to do this but we miss out on the timeStr and timeDetailStr so we
// give the user the option.
// aStream is one of ci or nightly.
func (g sippyDBPayloadGetter) getUrls(aVersion, aStream string) []ReleasePayload {
	sippyUrl := "https://sippy.dptools.openshift.org/api/releases/tags?&release=%s"
	body, err := getBodyTimeout(fmt.Sprintf(sippyUrl, aVersion), BODY_TIMEOUT)
	var ret []ReleasePayload
	if err != nil {
		if err == errDownloadTookTooLong {
			fmt.Println("Download problem on sippy API for releases")
			goutils.CheckErrFatal(err)
		}
	}
	type releaseItem struct {
		Release_tag    string
		Stream         string // ci or nightly
		Architecture   string
		Phase          string // Accepted or Rejected
		Forced         bool
		Release_time   string
		FailedJobNames []string // sippy returns a list of failed jobs
	}
	releaseList := []releaseItem{}
	err = json.Unmarshal(body, &releaseList)
	if err != nil {
		if err == errDownloadTookTooLong {
			fmt.Println("Problem unmarshalling sippy releases data into a releaseItem struct")
			goutils.CheckErrFatal(err)
		}
	}

	for _, relItem := range releaseList {
		if relItem.Architecture != "amd64" || relItem.Stream != aStream {
			continue
		}
		originallyFailed := false
		// If this payload failed an aggregated test, this payload originally failed
		for _, failedJobs := range relItem.FailedJobNames {
			if strings.HasPrefix(failedJobs, "aggregated") {
				originallyFailed = true
			}
		}
		if relItem.Phase == acceptedStr && originallyFailed {
			relItem.Forced = true
		}
		ret = append(ret, ReleasePayload{
			ReleaseURL: fmt.Sprintf("%s/releasestream/%s.0-0.%s/release/%s", releaseUrlPrefix, aVersion, aStream, relItem.Release_tag),
			phase:      relItem.Phase,
			forced:     relItem.Forced,
			timeStr:    relItem.Release_time,
		})
	}
	return ret
}

// getUrls fetches the urls from the release-controller api.  This is a cleaner way to
// do this but we miss out on the timeStr and timeDetailStr so we give the user the option.
func (g rcAPIPayloadGetter) getUrls(aVersion, aStream string) []ReleasePayload {
	const relContStr = "https://amd64.ocp.releases.ci.openshift.org/api/v1/releasestream/%s.0-0.%s/tags"
	releaseStr := fmt.Sprintf(relContStr, aVersion, aStream)
	fmt.Println(releaseStr)
	body, err := getBodyTimeout(releaseStr, BODY_TIMEOUT)
	var ret []ReleasePayload
	if err != nil {
		if err == errDownloadTookTooLong {
			fmt.Println("Error talking to release-controller api")
			goutils.CheckErrFatal(err)
		}
		goutils.CheckErrFatal(err)
	}

	type tag struct {
		Name        string // 4.14.0-0.nightly-2023-03-19-193640
		Phase       string // Accepted or Rejected
		Pullspec    string // registry.ci.openshift.org/ocp/release:4.14.0-0.nightly-2023-03-19-193640
		DownloadURL string // https://openshift-...2s4.p1.../4.14.0-0.nightly-2023-03-19-193640
	}
	type rcReleaseItems struct {
		Name string // 4.14.0-0.nightly or 4.14.0-0.ci
		Tags []tag
	}
	releaseList := rcReleaseItems{}
	err = json.Unmarshal(body, &releaseList)
	if err != nil {
		if err == errDownloadTookTooLong {
			fmt.Println("Error unmarshalling release-controller output")
			goutils.CheckErrFatal(err)
		}
	}
	for _, relItem := range releaseList.Tags {
		ret = append(ret, ReleasePayload{
			ReleaseURL: fmt.Sprintf("%s/releasestream/%s.0-0.%s/release/%s", releaseUrlPrefix, aVersion, aStream, relItem.Name),
			phase:      relItem.Phase,
			timeStr:    "Unknown ago", // TODO: you can maybe calculate from the Name vs. time.Now()
		})
	}
	return ret
}

func getPayloadItems(releaseVersion, releaseStream string, dbMode dbModeType) []ReleasePayload {
	var payloadItems []ReleasePayload
	var payloadGetter payloadGetter

	switch dbMode {
	case rcWebpage:
		payloadGetter = rcWebpagePayloadGetter{}
	case sippyDB:
		payloadGetter = sippyDBPayloadGetter{}
	case rcAPI:
		payloadGetter = rcAPIPayloadGetter{}
	default:
		fmt.Println("Something is broken")
	}

	payloadItems = payloadGetter.getUrls(releaseVersion, releaseStream)

	return payloadItems
}

func processPayloadItems(payloadItems []ReleasePayload, showAllUrl, showAggrTimes, showSuccess, printTestDetail, showAggrJobDetail bool) {
	for _, payloadItem := range payloadItems {
		ProcessPayloadItem(payloadItem, showAllUrl, showAggrTimes, showSuccess, printTestDetail, showAggrJobDetail)
	}
}
