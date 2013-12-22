// main_test.go
package main

import (
	"log"
	"testing"
	"time"
)

func TestYqlExtractResponseArray(t *testing.T) {
	hist := make([]yqlHistory, 0, 1)

	// simulate YQL query response body:
	body := `{"query":{"count":1,"created":"2013-12-22T05:22:05Z","lang":"en-US","results":{"quote":[{"Symbol":"MSFT","Close":"36.80","Volume":"62649100","Date":"2013-12-20","Open":"36.20","High":"36.93","Low":"36.19"}]}}}`

	// Test decoding the JSON:
	if err := yqlExtractResponse([]byte(body), &hist, nil); err != nil {
		log.Println(err)
		return
	}

	log.Println(hist)
}

func TestYqlExtractResponseObject(t *testing.T) {
	hist := make([]yqlHistory, 0, 1)

	// simulate YQL query response body:
	body := `{"query":{"count":1,"created":"2013-12-22T05:22:05Z","lang":"en-US","results":{"quote":{"Symbol":"MSFT","Close":"36.80","Volume":"62649100","Date":"2013-12-20","Open":"36.20","High":"36.93","Low":"36.19"}}}}`

	// Test decoding the JSON:
	if err := yqlExtractResponse([]byte(body), &hist, nil); err != nil {
		log.Println(err)
		return
	}

	log.Println(hist)
}

func TestTruncDate(t *testing.T) {
	// Get the New York location for stock timezone:
	nyLoc, _ := time.LoadLocation("America/New_York")

	today := truncDate(time.Now().In(nyLoc))
	yesterday := today.Add(time.Hour * time.Duration(-24))

	log.Println(today)
	log.Println(yesterday)
}
