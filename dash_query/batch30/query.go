package main

import (
	"bufio"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	_ "github.com/couchbase/go_n1ql"
	"log"
	"os"
	"runtime"
	"sync"
	"time"
)

var serverURL = flag.String("server", "http://localhost:8093",
	"couchbase server URL")
var threads = flag.Int("threads", 10, "number of threads")
var queryFile = flag.String("queryfile", "query_file.txt", "file containing list of select queries")
var diff = flag.Int("diff", 100, "time difference")
var lag = flag.Int("lag", 60, "time lag in seconds")

var wg sync.WaitGroup

func main() {

	flag.Parse()

	// set GO_MAXPROCS to the number of threads
	runtime.GOMAXPROCS(*threads)

	queryLines, err := readLines(*queryFile)
	if err != nil {
		log.Fatal(" Unable to read from file %s, Error %v", *queryFile, err)
	}

	for i := 0; i < *threads; i++ {
		wg.Add(1)
		go runQuery(*serverURL, queryLines, i)
	}

	wg.Wait()
}

// readLines reads a whole file into memory
// and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	return lines, scanner.Err()
}

func runQuery(server string, queryLines []string, offset int) {

	n1ql, err := sql.Open("n1ql", *serverURL)
	if err != nil {
		log.Fatal(err)
	}

	err = n1ql.Ping()
	if err != nil {
		log.Fatal(err)
	}

	// Set query parameters
	os.Setenv("n1ql_timeout", "1000s")
	ac := []byte(`[{"user": "admin:Administrator", "pass": "asdasd"}]`)
	os.Setenv("n1ql_creds", string(ac))

	results := make([]interface{}, 0)

	for i, query := range queryLines {

		if i != offset {
			continue
		}

		var rows *sql.Rows

		startTime := time.Now()
		rows, err = n1ql.Query(query, startTime.Unix()-int64(*diff)-int64(*lag), startTime.Unix()-int64(*lag))
		if err != nil {
			log.Fatal("Error Query Line ", err, query, i)
		}

		defer rows.Close()
		cols, err := rows.Columns()
		if err != nil {
			log.Printf("No columns returned %v", err)
			break
		}
		if cols == nil {
			log.Printf("No columns returned")
			break
		}

		vals := make([]interface{}, len(cols))
		for i := 0; i < len(cols); i++ {
			vals[i] = new(interface{})
		}

		for rows.Next() {
			row := make(map[string]interface{})
			err = rows.Scan(vals...)
			if err != nil {
				fmt.Println(err)
				continue
			}
			for i := 0; i < len(vals); i++ {
				row[cols[i]] = returnValue(vals[i].(*interface{}))
			}
			results = append(results, row)

		}
		if rows.Err() != nil {
			log.Printf("Error sanning rows %v", err)
		}

		resultStr, _ := json.MarshalIndent(results, "", "    ")
		fmt.Printf("Query %v \n Result %s \n", query, resultStr)

	}

	wg.Done()
}

func returnValue(pval *interface{}) interface{} {
	switch v := (*pval).(type) {
	case nil:
		return "NULL"
	case bool:
		if v {
			return true
		} else {
			return false
		}
	case []byte:
		return string(v)
	case time.Time:
		return v.Format("2006-01-02 15:04:05.999")
	default:
		return v
	}
}
