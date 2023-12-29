# Release Analysis

This will help TRT watchers analyze release payloads and prow jobs from release payloads

## Usage

The `-d xxxx` option allows you to pull release payload tags (e.g., 4.15.0-0.nightly-2023-12-25-100326) from these places:

* [release-controller](https://amd64.ocp.releases.ci.openshift.org/) webpage via rudimentary scraping
  * this gets you the latest payloads (including those in progress)
* [sippy database](https://sippy.dptools.openshift.org/sippy-ng/)
  * this gets you all payloads persisted in sippy's DB (which will be more than in the release-controller)
  * data from here will be upto one hour old
* release-controller API
  * cleaner than webpage scraping but you will miss out on the times which show the age of the payloads

Examples for `payload`:

```bash
./release-analysis payload 4.15 nightly -d rcWebpage ;# rc webpage
./release-analysis payload 4.15 ci      -d sippyDB   ;# sippyDB
./release-analysis payload 4.15 nightly -d rcAPI     ;# rc API
./release-analysis payload 4.15 nightly -d rcWebpax  ;# bad argument, default to rcWebpage
```

Examples for `analysis`:

```bash
./release-analysis analysis https://amd64.ocp.releases.ci.openshift.org/releasestream/4.15.0-0.nightly/release/4.15.0-0.nightly-2023-12-23-011438
./release-analysis analysis https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/aggregated-gcp-ovn-rt-upgrade-4.16-minor-release-openshift-release-analysis-aggregator/1739449957754081280
```