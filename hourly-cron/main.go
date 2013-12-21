package main

import "fmt"
import "log"
import _ "errors"

import "encoding/json"
import "io/ioutil"
import "net/http"
import "net/url"

import "database/sql"
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

func db_create_schema() (db *sqlx.DB, err error) {
	// using sqlite 3.8.0 release
	db, err = sqlx.Connect("sqlite3", "stocks.db")
	if err != nil {
		db.Close()
		return
	}

	_, err = db.Exec(`
create table if not exists stock (
	symbol TEXT UNIQUE NOT NULL,
	purchase_price TEXT NOT NULL,
	purchase_date TEXT NOT NULL,
	purchaser_email TEXT NOT NULL,
	trailing_stop_percent TEXT NOT NULL,
	last_stop_price TEXT
)`)
	if err != nil {
		db.Close()
		return
	}

	_, err = db.Exec(`
create table if not exists stock_history (
	symbol TEXT NOT NULL,
	date TEXT NOT NULL,
	closing_price TEXT NOT NULL
)`)
	if err != nil {
		db.Close()
		return
	}

	// Add some test data:
	db.Exec(`
insert into stock  (symbol, purchase_price, purchase_date, purchaser_email, trailing_stop_percent, last_stop_price)
 			values ('MSFT', '40.00', '2013-12-01', 'email@example.org', '20.00', NULL)
`)

	return
}

type Stock struct {
	Symbol              string         `db:"symbol"`
	PurchasePrice       string         `db:"purchase_price"`
	PurchaseDate        string         `db:"purchase_date"`
	PurchaserEmail      string         `db:"purchaser_email"`
	TrailingStopPercent string         `db:"trailing_stop_percent"`
	LastStopPrice       sql.NullString `db:"last_stop_price"`
}

// main:
func main() {
	// Create our DB schema:
	db, err := db_create_schema()
	if err != nil {
		log.Fatal(err)
		return
	}
	defer db.Close()

	// Query stocks table:
	stocks := make([]Stock, 0, 4) // make(type, len, capacity)
	err = db.Select(&stocks, `select symbol, purchase_price, purchase_date, purchaser_email, trailing_stop_percent, last_stop_price from stock`)
	if err != nil {
		log.Fatal(err)
		return
	}
	fmt.Printf("%#v\n", stocks[0])

	// get current price of MSFT:
	quot := new(QuoteResponse)
	err = yql(quot, `select symbol, LastTradePriceOnly from yahoo.finance.quote where symbol in ("MSFT")`)
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Printf("%#v\n", *quot.Query.Results.Quote)

	// get historical data for MSFT:
	hist := new(HistoricalResponse)
	err = yql(hist, `select Date, Close from yahoo.finance.historicaldata where symbol = "MSFT" and startDate = "2013-12-04" and endDate = "2013-12-06"`)
	if err != nil {
		log.Fatal(err)
		return
	}

	fmt.Printf("%#v %#v %#v\n", *hist.Query.Results.Quote[0], *hist.Query.Results.Quote[1], *hist.Query.Results.Quote[2])
	return
}
