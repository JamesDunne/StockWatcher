package main

import "fmt"

import "encoding/json"
import "io/ioutil"
import "net/http"
import "net/url"
import "database/sql"
import _ "github.com/mattn/go-sqlite3"

// `q` is the YQL query
func yql(jrsp interface{}, q string) (err error) {
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
			Quote *Quote "quote"
		} "results"
	} "query"
}

type Historical struct {
	Date  string
	Close string
}

type HistoricalResponse struct {
	Query *struct {
		Results *struct {
			Quote []*Historical "quote"
		} "results"
	} "query"
}

func db_create_schema(db *sql.DB) (err error) {
	_, err = db.Exec(`create table if not exists stock (symbol TEXT UNIQUE, purchase_price NUMERIC, purchase_date TEXT, trailing_stop_percent NUMERIC, last_stop_price NUMERIC)`)
	if err != nil {
		return
	}

	_, err = db.Exec(`create table if not exists stock_history (symbol TEXT, date TEXT, closing_price NUMERIC)`)
	if err != nil {
		return
	}

	return
}

// main:
func main() {
	// using sqlite 3.8.0 release
	db, err := sql.Open("sqlite3", "stocks.db")
	if err != nil {
		fmt.Println(err)
		return
	}
	defer db.Close()

	// Create our schema:
	err = db_create_schema(db)
	if err != nil {
		fmt.Println(err)
		return
	}

	_, err = db.Query(`select * from stock`)
	if err != nil {
		fmt.Println(err)
		return
	}

	// get current price of MSFT:
	quot := new(QuoteResponse)
	err = yql(quot, `select symbol, LastTradePriceOnly from yahoo.finance.quote where symbol in ("MSFT")`)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("%s\n", *quot.Query.Results.Quote)

	// get historical data for MSFT:
	hist := new(HistoricalResponse)
	err = yql(hist, `select Date, Close from yahoo.finance.historicaldata where symbol = "MSFT" and startDate = "2013-12-04" and endDate = "2013-12-06"`)
	if err != nil {
		fmt.Println(err)
		return
	}

	fmt.Printf("%s %s %s\n", *hist.Query.Results.Quote[0], *hist.Query.Results.Quote[1], *hist.Query.Results.Quote[2])
	return
}
