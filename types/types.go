package payload_process

// phase: Accepted or Rejected (annoying name hence this comment, but we match sippy style)
type ReleasePayload struct {
	ReleaseURL    string // e.g., https://amd64.ocp.releases.ci.openshift.org/releasestream/4.14.0-0.nightly/release/4.14.0-0.nightly-2023-03-11-044613
	phase         string // Accepted or Rejected
	forced        bool   // in case it was forced accepted or rejected (filled only if we use getUrlsFromSippy) and not so accurate anyway
	timeStr       string // e.g., "4 days ago" or "31 hours ago"
	timeDetailStr string // e.g., 03-11T04:46:13Z
}