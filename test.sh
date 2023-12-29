./release-analysis payload 4.15 nightly -d rcWebpage ;# rc webpage
./release-analysis payload 4.15 nightly -d sippyDB   ;# sippyDB
./release-analysis payload 4.15 nightly -d rcAPI 0   ;# rc API
./release-analysis payload 4.15 nightly -d rcWebpax  ;# bad argument, default to rcWebpage

./release-analysis analysis https://amd64.ocp.releases.ci.openshift.org/releasestream/4.15.0-0.nightly/release/4.15.0-0.nightly-2023-12-23-011438
./release-analysis analysis https://prow.ci.openshift.org/view/gs/origin-ci-test/logs/aggregated-gcp-ovn-rt-upgrade-4.16-minor-release-openshift-release-analysis-aggregator/1739449957754081280

