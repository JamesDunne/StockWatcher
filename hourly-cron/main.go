package main

import "fmt"

import "encoding/json"
import "io/ioutil"
import "net/http"
import "net/url"

// `q` is the YQL query
func yql(q string, jrsp interface{}) (err error) {
	// form the YQL URL:
	u := `http://query.yahooapis.com/v1/public/yql?q=` + url.QueryEscape(q) + `&format=json&env=store%3A%2F%2Fdatatables.org%2Falltableswithkeys`
	resp, err := http.Get(u)
	if err != nil {
		return
	}

	// read body:
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
	}

	// print JSON to console:
	//fmt.Printf("%s\n\n", body)

	// decode JSON:
	err = json.Unmarshal(body, jrsp)
	return
}

// Head to http://developer.yahoo.com/yql/console/?q=select%20*%20from%20yahoo.finance.quote%20where%20symbol%20in%20(%22YHOO%22%2C%22AAPL%22%2C%22GOOG%22%2C%22MSFT%22)&env=store%3A%2F%2Fdatatables.org%2Falltableswithkeys
// to understand this JSON structure.

type Quote struct {
	Symbol             string "symbol"
	LastTradePriceOnly string
}

type QuoteResponse struct {
	Query *struct {
		Results *struct {
			Quote *Quote
		}
	}
}

// main:
func main() {
	// get current price of MSFT:
	jrsp := new(QuoteResponse)
	err := yql(`select * from yahoo.finance.quote where symbol in ("MSFT")`, jrsp)
	if err != nil {
		fmt.Print(err)
		return
	}

	fmt.Printf("%s\n", *jrsp.Query.Results.Quote)

	// TODO: get historical data:
	//`select * from yahoo.finance.historicaldata where symbol = "MSFT" and startDate = "2013-12-04" and endDate = "2013-12-06"`
	return
}
