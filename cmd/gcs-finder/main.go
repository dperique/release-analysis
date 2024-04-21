package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"regexp"
	"strings"

	"cloud.google.com/go/storage"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

const (
	// ANSI color codes
	redStart   = "\033[31m"
	colorReset = "\033[0m"
	lightGreen = "\033[92m"
	yellow     = "\033[33m"

	prowGCSBucket = "test-platform-results"
)

// TIP: use this in a VScode terminal and the links, produced by --showUrls, are clickable.

func main() {
	// Parse the input parameters

	// We can give one of these:
	// 1. The path to the artifacts (as copied from the Artifacts link in a ProwJob)
	// 2. The jobName and jobID separately (which will be used to craft an Artifacts path)
	// We allow number 2 as a convenience because sometimes we have a ProwJobPath but it
	// doesn't load and all we care about is the Artifacts link (and having a jobName and jobID
	// is enough to craft the Artifacts path).

	inputPath := flag.String("path", "", "Artifacts path from the prowJob's Artifacts link")

	inputJobName := flag.String("jobName", "", "Job name")
	inputJobID := flag.String("jobID", "", "Job ID")

	inputRegex := flag.String("regex", "", "Regex to filter the files")
	inputGcsCredentials := flag.String("cred", "", "Path to the GCS credentials file; or use GOOGLE_APPLICATION_CREDENTIALS")

	inputShowUrls := flag.Bool("showUrls", false, "Show the full URLs of the files; green is the file, yellow is the containing directory")

	help := flag.Bool("help", false, "Prints the help message")

	flag.Parse()

	if *help {
		flag.PrintDefaults()
		return
	}

	if *inputJobName == "" || *inputJobID == "" {
		if *inputPath == "" {
			log.Fatal("Please provide a path using -path flag or a job name and job ID using -jobName and -jobID flags")
		}
	}
	if *inputPath == "" {
		if *inputJobName == "" || *inputJobID == "" {
			log.Fatal("Please provide a job name and job ID using -jobName and -jobID flags")
		}

		// Craft an inputPath from the jobName and jobID
		*inputPath = fmt.Sprintf("https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/logs/%s/%s", *inputJobName, *inputJobID)
	}
	if *inputRegex == "" {
		log.Fatal("Please provide a regex using -regex flag")
	}

	if inputGcsCredentials == nil {

		// If the user passed in the creds as an env variable, use that
		if creds := strings.TrimSpace(flag.Lookup("GOOGLE_APPLICATION_CREDENTIALS").Value.String()); creds != "" {
			inputGcsCredentials = &creds
		} else {
			log.Fatal("Please provide the path to the GCS credentials file using -gcs-credentials flag")
		}
	}

	regex, err := regexp.Compile(*inputRegex)
	if err != nil {
		log.Fatalf("Invalid regex: %v", err)
	}

	// Take the input path and remove https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/logs/ prefix
	jobPath := strings.TrimPrefix(*inputPath, "https://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/logs/")

	// Extract the jobName and jobID from the path
	var jobName, jobID string
	if *inputJobName != "" && *inputJobID != "" {
		// If we got here, the user passed in a jobName and jobID
		jobName = *inputJobName
		jobID = *inputJobID
	} else {
		// If we got here, the user passed in a path
		parts := strings.Split(jobPath, "/")
		if len(parts) != 3 {
			log.Fatalf("Invalid path: %s", jobPath)
		}
		jobName = parts[0]
		jobID = parts[1]
	}

	// Initialize GCS client and authenticate
	ctx := context.Background()
	client, err := storage.NewClient(ctx, option.WithCredentialsFile(*inputGcsCredentials))
	if err != nil {
		log.Fatalf("Failed to create client using %s: %v", *inputGcsCredentials, err)
	}
	defer client.Close()

	// Set up the query to list files in the specified path.  The Prefix needs to look
	// like this so you can start from the root of artifacts for the prowJob:
	// "logs/periodic-ci-openshift-release-master-ci-4.16-upgrade-from-stable-4.15-e2e-gcp-ovn-rt-upgrade/1737738676454035456"
	query := &storage.Query{Prefix: "logs/" + jobPath}

	// Only retrieve the name and size to make the searches fast as we don't
	// care about the contents.
	if err := query.SetAttrSelection([]string{"Name", "Size"}); err != nil {
		log.Fatalf("Failed to set attribute selection: %v", err)
	}

	bucketHandle := client.Bucket(prowGCSBucket)
	it := bucketHandle.Objects(ctx, query)

	// Iterate through the objects in the GCS bucket
	fmt.Printf("Listing files in: %s\n\n", jobPath)
	totalFiles := 0
	totalMatches := 0
	for {
		attrs, err := it.Next()
		if err == iterator.Done {
			break
		}
		totalFiles++
		if err != nil {
			log.Fatalf("Failed to list objects: %v", err)
		}
		name := strings.TrimPrefix(attrs.Name, "logs/"+jobName+"/"+jobID+"/")

		match := regex.FindString(attrs.Name)
		if match != "" {
			// Replace the match in the original string with the colored version
			coloredMatch := redStart + match + colorReset
			coloredName := strings.Replace(name, match, coloredMatch, 1)

			// Turn the attr.Size into a human readable format
			fmt.Printf("%10s %s\n", formatBytes(attrs.Size), coloredName)

			if inputShowUrls != nil && *inputShowUrls {
				// Craft the full URL of the file we found
				fullURL := fmt.Sprintf("%10s %shttps://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/logs/%s/%s/%s%s", " ", lightGreen, jobName, jobID, name, colorReset)

				// Get the dirName by trimming starting from the last slash
				dirName := name[:strings.LastIndex(name, "/")]

				dirURL := fmt.Sprintf("%10s %shttps://gcsweb-ci.apps.ci.l2s4.p1.openshiftapps.com/gcs/test-platform-results/logs/%s/%s/%s%s", " ", yellow, jobName, jobID, dirName, colorReset)
				fmt.Printf("  %s\n", fullURL)
				fmt.Printf("  %s\n", dirURL)
			}
			totalMatches++
		}
	}
	// Print the count
	fmt.Println()
	fmt.Printf("Total files: %d, Total matches: %d\n", totalFiles, totalMatches)
}

// formatBytes converts bytes to a human-readable string format
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d.0 B", bytes) // No decimal for bytes
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %c", float64(bytes)/float64(div), "KMGTPE"[exp])
}
