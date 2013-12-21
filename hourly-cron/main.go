package main

import "fmt"
import "log"
import _ "errors"
import "os"

import "encoding/json"
import "io/ioutil"
import "net/http"
import "net/url"

import _ "database/sql"
import _ "github.com/mattn/go-sqlite3"
import "github.com/jmoiron/sqlx"

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

	// Need a 200 response:
	if resp.StatusCode != 200 {
		err = fmt.Errorf("%s", resp.Status)
		return
	}

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

func db_create_schema(path string) (db *sqlx.DB, err error) {
	// using sqlite 3.8.0 release
	db, err = sqlx.Connect("sqlite3", path)
	if err != nil {
		db.Close()
		return
	}

	if _, err = db.Exec(`
create table if not exists stock_track (
	symbol TEXT NOT NULL,
	purchase_price TEXT NOT NULL,
	purchase_date TEXT NOT NULL,
	purchaser_email TEXT NOT NULL,
	trailing_stop_percent TEXT NOT NULL
)`); err != nil {
		db.Close()
		return
	}

	if _, err = db.Exec(`
create table if not exists stock_history (
	symbol TEXT NOT NULL,
	date TEXT NOT NULL,
	closing_price TEXT NOT NULL
)`); err != nil {
		db.Close()
		return
	}

	// Add some test data:
	db.Execl(`
insert into stock_track (symbol, purchase_price, purchase_date, purchaser_email, trailing_stop_percent)
            values ('MSFT', '30.00', '2013-12-01', 'email@example.org', '20.00')
`)
	db.Execl(`
insert into stock_history (symbol, date, closing_price)
            values        ('MSFT', '2013-12-20', '36.80')
`)

	return
}

type dbStockPrice struct {
	Symbol              string `db:"symbol"`
	PurchasePrice       string `db:"purchase_price"`
	PurchaseDate        string `db:"purchase_date"`
	PurchaserEmail      string `db:"purchaser_email"`
	TrailingStopPercent string `db:"trailing_stop_percent"`
	LastCloseDate       string `db:"closing_date"`
	LastClosePrice      string `db:"closing_price"`
}

// main:
func main() {
	const dbPath = "./stocks.db"

	// Create our DB file and its schema:
	os.Remove(dbPath)
	db, err := db_create_schema(dbPath)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer db.Close()

	// Query stocks table:
	stocks := make([]dbStockPrice, 0, 4) // make(type, len, capacity)
	if err = db.Select(&stocks, `
select s.symbol, s.purchase_price, s.purchase_date, s.purchaser_email, s.trailing_stop_percent, h.date as closing_date, h.closing_price
from stock_track as s
left join stock_history as h on h.symbol = s.symbol and h.date = (select max(date) from stock_history where symbol = s.symbol)
`); err != nil {
		log.Fatal(err)
	}

	for i, value := range stocks {
		if i == 0 {
			fmt.Printf("%#v", value)
		} else {
			fmt.Printf(", %#v", value)
		}
	}
	fmt.Println()

	// get current price of MSFT:
	quot := new(QuoteResponse)
	err = yql(quot, `select Symbol, LastTradePriceOnly from yahoo.finance.quote where symbol in ("MSFT")`)
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Printf("%#v\n", *quot.Query.Results.Quote)

	// get historical data for MSFT:
	hist := new(HistoricalResponse)
	if err = yql(hist, `select Date, Close from yahoo.finance.historicaldata where symbol = "MSFT" and startDate = "2013-12-16" and endDate = "2013-12-20"`); err != nil {
		log.Fatal(err)
		return
	}

	for i, value := range hist.Query.Results.Quote {
		if i == 0 {
			fmt.Printf("%#v", *value)
		} else {
			fmt.Printf(", %#v", *value)
		}
	}
	fmt.Println()
	return
}
