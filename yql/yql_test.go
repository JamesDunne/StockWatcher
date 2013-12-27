package yql

import (
	"fmt"
	"testing"
	//	"time"
)

func TestYqlExtractResponseArray(t *testing.T) {
	hist := make([]History, 0, 1)

	// simulate YQL query response body:
	body := `{"query":{"count":1,"created":"2013-12-22T05:22:05Z","lang":"en-US","results":{"quote":[{"Symbol":"MSFT","Close":"36.80","Volume":"62649100","Date":"2013-12-20","Open":"36.20","High":"36.93","Low":"36.19"}]}}}`

	// Test decoding the JSON:
	if err := extractResponse([]byte(body), &hist, nil); err != nil {
		t.Fatal(err)
		return
	}

	fmt.Println(hist)
}

func TestYqlExtractResponseObject(t *testing.T) {
	hist := make([]History, 0, 1)

	// simulate YQL query response body:
	body := `{"query":{"count":1,"created":"2013-12-22T05:22:05Z","lang":"en-US","results":{"quote":{"Symbol":"MSFT","Close":"36.80","Volume":"62649100","Date":"2013-12-20","Open":"36.20","High":"36.93","Low":"36.19"}}}}`

	// Test decoding the JSON:
	if err := extractResponse([]byte(body), &hist, nil); err != nil {
		t.Fatal(err)
		return
	}

	fmt.Println(hist)
}

func TestGetQuote(t *testing.T) {
	quote, err := GetQuote("MSFT")
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("quote: %+v\n", quote)
}

func TestGetQuotesEmpty(t *testing.T) {
	quotes, err := GetQuotes()
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("quotes: %+v\n", quotes)
}

func TestGetQuotesSingle(t *testing.T) {
	quotes, err := GetQuotes("MSFT")
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("quotes: %+v\n", quotes)
}

func TestGetQuotesMulti(t *testing.T) {
	quotes, err := GetQuotes("MSFT", "AAPL")
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("quotes: %+v\n", quotes)
}

//func TestGetHistory(t *testing.T) {
//	startDate, err := time.Parse(dateFmt, "2011-11-26")
//	endDate, err := time.Parse(dateFmt, "2013-12-25")
//	res, err := GetHistory("MSFT", startDate, endDate)
//	if err != nil {
//		t.Fatal(err)
//		return
//	}

//	//_ = res
//	for _, r := range res {
//		fmt.Println(r.Date)
//	}
//}
