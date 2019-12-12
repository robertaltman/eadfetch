package main

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type archonCollection struct {
	ID                   int
	CollectionIdentifier string
	Title                string
	URL                  string
}

// checkSourceAndDest takes validates the existence of source and destination locations.
// returns the type of file, the []byte data contained in the file, and errors.
func checkSourceAndDest(srcFilePath string, dstDir string) (string, []byte, error) {
	// Check for valid file type.
	var fileType string
	if strings.Contains(srcFilePath, "csv") {
		fileType = "csv"
	} else if strings.Contains(srcFilePath, "json") {
		fileType = "json"
	} else {
		return fileType, nil, errors.New("cannot detect file type. please make sure the proper extension is appended to the filename")
	}

	// // Open input file. Exit if error.
	fdata, err := ioutil.ReadFile(srcFilePath)
	if err != nil {
		return fileType, nil, err
	}

	// Verify that the output directory exists. If not, create.
	_, err = os.Stat(dstDir)
	if err != nil {
		err = os.MkdirAll(dstDir, 0755)
		if err != nil {
			return fileType, nil, err
		}
	}
	return fileType, fdata, nil
}

func unmarshalRecords(fileType string, fileData []byte) ([]archonCollection, error) {
	var arCollections []archonCollection

	// Alter process for unpacking/unmarshalling of data into arCollections depending on file type.
	if fileType == "json" {
		json.Unmarshal(fileData, &arCollections)
	} else {
		fd := csv.NewReader(bytes.NewReader(fileData))
		rows, err := fd.ReadAll()
		if err != nil {
			return arCollections, err
		}

		err = csvToCollections(rows, &arCollections)
		if err != nil {
			return arCollections, err
		}
	}
	return arCollections, nil
}

// addURL builds a proper URL for retrieving the EAD output from Archon
// e.g., https://beckerarchives.wustl.edu/index.php?p=collections/ead&templateset=ead&disabletheme=1&id=2
func addURL(dn string, ac archonCollection) string {
	urlString := "/index.php?p=collections/ead&templateset=ead&disabletheme=1&id="
	return fmt.Sprintf("%s%s%d", dn, urlString, ac.ID)
}

// getEAD receives collections on the requests queue channel, saves EAD to file, and sends results data
// on a output channel for the writeURLCSVReport function to use later.
func getEAD(timeout int, rateLimiter *time.Ticker, rQueue chan archonCollection, resultsChan chan<- []string, wg *sync.WaitGroup, output string, nameflags string) {

	for r := range rQueue {
		<-rateLimiter.C

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
		defer cancel()

		var requestError string
		request, err := http.NewRequestWithContext(ctx, "GET", r.URL, nil)
		if err != nil {
			requestError = fmt.Sprintf("Request Error: %s", err)
		}

		res, err := http.DefaultClient.Do(request)
		if err != nil {
			if strings.Contains(err.Error(), "context deadline exceeded") {
				log.Printf("Error matches context issue: %s", err)
				rQueue <- r
				// Cancel context
				cancel()
				continue
			}
			requestError = fmt.Sprintf("Request Error: %s", err)
			fmt.Print(requestError)
			resultsChan <- []string{r.URL, r.CollectionIdentifier, res.Status, requestError}
			// Cancel context
			cancel()
			continue
		}

		eadxml, err := ioutil.ReadAll(res.Body)
		defer res.Body.Close()
		if err != nil {
			if strings.Contains(err.Error(), "context deadline exceeded") {
				rQueue <- r
				// Cancel context
				cancel()
				continue
			}

			requestError = fmt.Sprintf("Error reading response body. Adding back to request queue: %s", err)
			resultsChan <- []string{r.URL, r.CollectionIdentifier, res.Status, requestError}
			// Cancel context
			cancel()
			continue
		}

		if bytes.Contains(eadxml, []byte("Could not load Collection: Collection ID")) {
			requestError = fmt.Sprintf("Request Error: %s", eadxml)
		} else {
			name := generateFilename(r, nameflags)
			eadfilename := output + "/" + name + ".xml"
			err = ioutil.WriteFile(eadfilename, eadxml, 0644)
			if err != nil {
				log.Fatal(err)
			}
		}

		resultsChan <- []string{r.URL, r.CollectionIdentifier, res.Status, requestError}

		// Context cancelled via defer at beginning of loop.
	}
	defer wg.Done()
}

func writeURLCSVReport(outputPath string, repData [][]string) error {
	reportFile, err := os.Create(outputPath + "/ead-fetch-report.csv")
	defer reportFile.Close()
	if err != nil {
		return err
	}

	csvWriter := csv.NewWriter(reportFile)
	err = csvWriter.WriteAll(repData)
	if err != nil {
		return err
	}
	return nil
}

// fieldMapper processes a slice containing the first/header row from a CSV file and
// attempts to map them to the correct archonCollection struct field.
func fieldMapper(csvHeader []string) (map[string]int, error) {
	fieldMap := make(map[string]int)
	fields := []string{
		"ID",
		"CollectionIdentifier",
		"Title",
	}

	for _, v := range fields {
		for key, csvColumn := range csvHeader {
			// This is to strip out any byte order marks (BOM) from the header row.
			csvColumn = strings.TrimFunc(csvColumn, func(r rune) bool {
				return !unicode.IsLetter(r)
			})
			// Compare column value to fields slice, insensitive to case. Update field map.
			if strings.EqualFold(v, csvColumn) {
				fieldMap[v] = key
				break
			}
		}
	}

	if _, ok := fieldMap["ID"]; ok {
		return fieldMap, nil
	}
	return nil, errors.New("no ID column found in CSV")
}

// csvToCollections processes rows of CSV data and stores the result in the value
// pointed to by collections.
func csvToCollections(csvRows [][]string, collections *[]archonCollection) error {
	fieldMap, err := fieldMapper(csvRows[0])
	if err != nil {
		return err
	}
	for _, row := range csvRows[1:] {
		id, err := strconv.Atoi(row[fieldMap["ID"]])
		if err != nil {
			return err
		}
		newCollection := archonCollection{
			ID:                   id,
			CollectionIdentifier: row[fieldMap["CollectionIdentifier"]],
			Title:                row[fieldMap["Title"]],
		}
		*collections = append(*collections, newCollection)
	}
	return nil
}

func generateFilename(collection archonCollection, nameFlag string) string {
	// Process flag. If flag is empty, just return 'ead_' concatenated with collection.ID
	if nameFlag == "" {
		return "ead_" + strconv.Itoa(collection.ID)
	}
	ctype := reflect.ValueOf(collection)
	namePartOrder := strings.Split(nameFlag, ",")

	var nameParts []string
	for _, v := range namePartOrder {
		part := reflect.Indirect(ctype).FieldByName(v)
		partStr := part.String()
		if partStr == "" {
			continue
		}
		nameParts = append(nameParts, partStr)
	}

	if len(nameParts) == 0 {
		return "ead_" + strconv.Itoa(collection.ID)
	}

	name := sanitizeFilename(strings.Join(nameParts, "__"))
	return name + "_" + strconv.Itoa(collection.ID)
}

// sanitizeFilename replaces problematic characters with an underscore.
func sanitizeFilename(name string) string {
	regExclude := regexp.MustCompile("[`|!|@|#|$|%|^|&|*|(|)|+|=|{|}|[|]|\\||:|;|\"|<|>|/|\\?|\\,|\\.|\\s]*")
	// Strip excluded characters
	name = string(regExclude.ReplaceAll([]byte(name), []byte("_")))
	return name
}

func main() {

	// Input flags
	filePath := flag.String("file", "collections-table.csv", "File to parse for Archon data.")
	host := flag.String("host", "http://127.0.0.1", "Hostname of Archon site (e.g., https://archon-site.edu).")
	workers := flag.Int("workers", 2, "Number of request workers to initiate.")
	output := flag.String("output", "./ead_output", "Output directory for fetched XML.")
	timeout := flag.Int("timeout", 30, "Number of seconds to allow a request to take.")
	throttle := flag.Int("ratelimit", 4, "Limit for the number of requests to be made per second.")
	nameformat := flag.String("eadname", "", "Formatting options for naming the downloaded EAD files. \nSelect from one or both in any order, separated by commas: CollectionIdentifier, Title. (e.g., -eadname Title,CollectionIdentifier). \nDefault will result in a file name of 'ead_<ID column value>.xml'.\nThe ID is always appendended to the filename to ensure they are distinct. Unsafe characters will be removed.")
	testrun := flag.Int("test", 0, "Specifiy a number for testing a limited number of collections from the input file.")

	flag.Parse()

	// Test that host is reachable. If not, exit.
	_, err := http.Get(*host)
	if err != nil {
		log.Fatalf("%s. Verify that the server is reachable.", err)
	}

	// Check that source file and destination directory exists and can be read. Get type of file and file data.
	fileType, fdata, err := checkSourceAndDest(*filePath, *output)
	if err != nil {
		log.Fatal(err)
	}

	// Unpack the records.
	arCollections, err := unmarshalRecords(fileType, fdata)
	if err != nil {
		log.Fatal(err)
	}

	// If testrun flag is set, limit slice size.
	if *testrun != 0 {
		arCollections = arCollections[:*testrun]
	}

	// For informational purposes only. When job is complete, a duration will be printed.
	timeStart := time.Now()

	// Create channels for the queue of requests to make and another to receive the responses.
	reqQueue := make(chan archonCollection)
	resultsChan := make(chan []string)

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		for _, v := range arCollections {
			v.URL = addURL(*host, v)
			reqQueue <- v
		}
	}()

	// Ticker to throttle requests to the server.
	rateTicker := time.NewTicker(time.Duration(1000/(*throttle)) * time.Millisecond)
	defer rateTicker.Stop()

	for i := 1; i <= *workers; i++ {
		wg.Add(1)
		go getEAD(*timeout, rateTicker, reqQueue, resultsChan, &wg, *output, *nameformat)
	}

	var resReturns [][]string
	for i := 1; i <= len(arCollections); i++ {
		resReturns = append(resReturns, <-resultsChan)
		fmt.Printf("\rFetching: %d of %d collections.", i, len(arCollections))
	}

	close(reqQueue)
	close(resultsChan)

	wg.Wait()

	timeEnd := time.Now()

	fmt.Printf("\nEAD XML fetching completed in %v.\nWriting report...\n", timeEnd.Sub(timeStart))

	err = writeURLCSVReport(*output, resReturns)
	if err != nil {
		log.Fatalf("Error writing report: %s. EAD files may have been retrieved, "+
			"but it may be difficult to determine the completeness of the work without the report. "+
			"Consider fixing the underlying issue and running the job again.", err)
	}

	exitMessage := fmt.Sprintf("The report is now complete. "+
		"Look for the file 'ead-fetch-report.csv' in the %s directory.\n"+
		"The last column will contain information about the status of a retrieval.", *output)

	fmt.Printf("%s\n", exitMessage)
}
