package main

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
)

// This program downloads and unzips the logs from a link to a GCS bucket.
// It uses web scraping to find the links to the logs.
// The logs are in gzip format, so we unzip them.
// The main value-added is parallel downloading and automatic unzipping
// because when you need to search node logs to troubleshoot an openshift
// problem, there are a lot of logs to download.

// getBody takes a url, and returns the body (i.e., contents).
// This is the same thing you get when you do curl -sk url.
// Any error is fatal.
func getBody(url string) []byte {
	resp, err := http.Get(url)
	if err != nil {
		log.Fatalln(err)
	}
	body, err := io.ReadAll(resp.Body)
	checkErr((err))
	return body
}

// checkError exits (fatally) if there's an error.
func checkErr(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func processNode(url string) {
	body := getBody(url)
	nodesDirList := strings.Split(string(body), "\n")

	regExp1 := regexp.MustCompile(`^.*<a href=\"`)
	regExp2 := regexp.MustCompile(`\"><.*$`)

	w := sync.WaitGroup{}
	for _, i := range nodesDirList {
		// gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/logs/
		if strings.Contains(i, "gcs/test-platform-results") && strings.Contains(i, "nodes") {
			line := regExp1.ReplaceAllString(i, "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com")
			line = regExp2.ReplaceAllString(line, "")
			lineParts := strings.Split(line, "/")
			nodeName := lineParts[len(lineParts)-2]
			fmt.Printf("Node: %s\n", nodeName)

			w.Add(1)
			go func(nodeName string) {
				// Get both the current and previous journals.

				defer w.Done()
				for _, logType := range []string{"journal", "journal-previous"} {
					outputLogfileName := nodeName + "-" + logType + ".log"
					fmt.Printf("  Getting log: %s\n", outputLogfileName)
					outputFile, err := os.Create(outputLogfileName)
					if err != nil {
						log.Fatalln(err)
					}
					defer outputFile.Close()

					// Get the logfile contents.
					logUrl := line + logType
					body = getBody(logUrl)

					// The contents are in gzip format so we unzip.
					reader := bytes.NewReader(body)
					gzipReader, err := gzip.NewReader(reader)
					if err != nil {
						log.Fatalln(err)
					}
					defer gzipReader.Close()
					unzippedLog, err := io.ReadAll(gzipReader)
					checkErr(err)

					// Write the unzip'ed contents to file.
					_, err = outputFile.Write(unzippedLog)
					checkErr(err)
				}
			}(nodeName)
		}
	}
	w.Wait()
	fmt.Println("Done downloading/unzip'ing")
}

func processJunit(url, pattern string) {
	body := getBody(url)
	fileList := strings.Split(string(body), "\n")

	regExp1 := regexp.MustCompile(`^.*<a href=\"`)
	regExp2 := regexp.MustCompile(`\"><.*$`)

	filePatternRegex := regexp.MustCompile(pattern)

	count := 0
	for _, i := range fileList {
		if strings.Contains(i, "gcs/test-platform-results") {
			line := regExp1.ReplaceAllString(i, "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com")
			line = regExp2.ReplaceAllString(line, "")
			lineParts := strings.Split(line, "/")
			fileName := lineParts[len(lineParts)-1]

			if !filePatternRegex.MatchString(fileName) {
				continue
			}
			fmt.Printf("  Getting: %s\n", fileName)
			outputFile, err := os.Create(fileName)
			if err != nil {
				log.Fatalln(err)
			}

			// Get the file contents.
			body = getBody(line)

			// Write the contents to file.
			_, err = outputFile.Write(body)
			checkErr(err)
			_ = outputFile.Close()
			count++
		}
	}
	if count == 0 {
		fmt.Println("No files matched the pattern")
	} else {
		fmt.Println("Done downloading")
	}
}

func main() {
	if len(os.Args) < 3 {
		fmt.Printf("Usage: %s aMode aUrl aJUnitRegExPattern\n", os.Args[0])
		fmt.Println("  mode = node or junit")
		fmt.Println("  url = the relevant url (gather-extra/artifacts/nodes or e2e/e2e/artifacts/e2e/junit)")
		os.Exit(0)
	}
	mode := os.Args[1]
	switch {
	case mode == "node":
		processNode(os.Args[2])
	case mode == "junit":
		if len(os.Args[3]) == 0 {
			fmt.Println("aJUnitRegExPattern cannot be empty; aborting.")
			os.Exit(1)
		}
		url := os.Args[2]
		pattern := os.Args[3]
		processJunit(url, pattern)
	default:
		fmt.Printf("Unknown mode: %s\n", mode)
	}
}
