# Release Analysis

This will help Redhat Openshift [TRT (Technical Release Team)](https://docs.ci.openshift.org/docs/release-oversight/the-technical-release-team/) watchers analyze release payloads and prow jobs from release payloads.

This command may expand to include other useful tools.

## Usage

The `-d xxxx` option allows you to pull release payload tags (e.g., 4.15.0-0.nightly-2023-12-25-100326) from these places:

* [release-controller](https://amd64.ocp.releases.ci.openshift.org/) webpage via rudimentary scraping
  * this gets you the latest payloads (including those in progress).  This is the default mode.
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

## Tips on Usage

This section contains some ways to use this tool.

One way to use the tool is within the vscode terminal.  The vscode terminal allows the abiilty to click on a line to navigate to it as well as the ability to search the output in as robust a fashion as searching the file (including case sensitivity and regex):

* Run this in a vscode terminal and all the links will be "clickable"
  * start vscode, bring up the terminal window at the bottom (control-backtick)
  * Command Pallete: shift-command-p ; select "Terminal: Move Terminal Into Editor Area"
  * Move the Terminal tab outside of the vscode main window (so it becomes a standalone window)
* Run the commands in a while loop so they refresh every 15m with the latest info
* Create a terminal tab for different versions (e.g., one terminal, two tabs (one with 4.16 nightly and another with 4.15 nightly)).
* run ./release-analysis analysis on an aggregated job for details
  * See a particular prowjob buildID that failed, click on it
  * Use command-f to search for that buildID in the output (it's already in the search box), click the matching link to view the prowjob