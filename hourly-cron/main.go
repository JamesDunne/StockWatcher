package main

import "fmt"
import "log"
import _ "errors"

//import "os"
import "math/big"

import "time"
import "encoding/json"
import "io/ioutil"
import "net/http"
import "net/url"

import "database/sql"
import _ "github.com/mattn/go-sqlite3"
import "github.com/jmoiron/sqlx"

// Head to http://developer.yahoo.com/yql/console/?q=select%20*%20from%20yahoo.finance.quote%20where%20symbol%20in%20(%22YHOO%22%2C%22AAPL%22%2C%22GOOG%22%2C%22MSFT%22)&env=store%3A%2F%2Fdatatables.org%2Falltableswithkeys
// to understand this JSON structure.

type Quote struct {
	Symbol             string `json:"Symbol"`
	LastTradePriceOnly string
}

type QuoteResponse struct {
	Query struct {
		CreatedDate string `json:"created"`
		Results     *struct {
			Quote Quote `json:"quote"`
		} `json:"results"`
	} `json:"query"`
}

type Historical struct {
	Date  string
	Close string
}

type HistoricalResponse struct {
	Query struct {
		CreatedDate string `json:"created"`
		Results     *struct {
			Quote []Historical `json:"quote"`
		} `json:"results"`
	} `json:"query"`
}

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
	purchaser_email TEXT NOT NULL,
	purchase_price TEXT NOT NULL,
	purchase_date TEXT NOT NULL,
	trailing_stop_percent TEXT NOT NULL,
	CONSTRAINT stock_track_pk PRIMARY KEY (symbol, purchaser_email)
)`); err != nil {
		db.Close()
		return
	}

	if _, err = db.Exec(`
create index if not exists stock_track_ix on stock_track (
	symbol ASC,
	purchaser_email ASC
)`); err != nil {
		db.Close()
		return
	}

	if _, err = db.Exec(`
create table if not exists stock_history (
	symbol TEXT NOT NULL,
	closing_date TEXT NOT NULL,
	closing_price TEXT NOT NULL,
	CONSTRAINT stock_history_pk PRIMARY KEY (symbol, closing_date)
)`); err != nil {
		db.Close()
		return
	}

	if _, err = db.Exec(`
create index if not exists stock_history_ix on stock_history (
	symbol ASC,
	closing_date DESC
)`); err != nil {
		db.Close()
		return
	}

	// Add some test data:
	db.Execl(`
insert or ignore into stock_track (symbol, purchaser_email, purchase_price, purchase_date, trailing_stop_percent)
            values ('MSFT', 'email@example.org', '30.00', '2013-12-01', '20.00')`)

	return
}

type dbStock struct {
	Symbol              string `db:"symbol"`
	PurchasePrice       string `db:"purchase_price"`
	PurchaseDate        string `db:"purchase_date"`
	PurchaserEmail      string `db:"purchaser_email"`
	TrailingStopPercent string `db:"trailing_stop_percent"`
}

type dbStockHistory struct {
	Symbol       string `db:"symbol"`
	ClosingDate  string `db:"closing_date"`
	ClosingPrice string `db:"closing_price"`
}

// remove the time component of a datetime to get just a date at 00:00:00
func toDate(t time.Time) time.Time {
	hour, min, sec := t.Clock()
	nano := t.Nanosecond()

	d := time.Duration(-(uint64(nano) + uint64(sec)*uint64(time.Second) + uint64(min)*uint64(time.Minute) + uint64(hour)*uint64(time.Hour)))
	return t.Add(d)
}

func toRat(v string) *big.Rat {
	rat := new(big.Rat)
	rat.SetString(v)
	return rat
}

// main:
func main() {
	const dbPath = "./stocks.db"
	const dateFmt = "2006-01-02"

	// Get the New York location for stock timezone:
	nyLoc, _ := time.LoadLocation("America/New_York")
	//fmt.Println(nyLoc)

	// Create our DB file and its schema:
	//os.Remove(dbPath)
	db, err := db_create_schema(dbPath)
	if err != nil {
		log.Fatal(err)
		return
	}
	defer db.Close()

	// Query stocks table:
	stocks := make([]dbStock, 0, 4) // make(type, len, capacity)
	if err = db.Select(&stocks, `
select s.symbol, s.purchase_price, s.purchase_date, s.purchaser_email, s.trailing_stop_percent
from stock_track as s`); err != nil {
		log.Fatal(err)
	}

	now := time.Now()
	fmt.Print("Stocks:\n")
	for _, st := range stocks {
		fmt.Printf("  %#v\n", st)
		fmt.Printf("  %s\n", st.Symbol)

		// Stock times/dates are in NYC timezone:
		yesterday := toDate(now.Add(time.Duration(time.Hour * 24 * -1)).In(nyLoc))

		// Determine the last-fetched date for the stock:
		var lastDate time.Time
		lastDateRec := new(dbStockHistory)
		err = db.Get(lastDateRec, `
select h.symbol, h.closing_date, h.closing_price
from stock_history as h
where date(h.closing_date) = (select max(date(closing_date)) from stock_history where symbol = h.symbol)
  and h.symbol = ?`, st.Symbol)
		if err == sql.ErrNoRows {
			// No row; use 7 days ago in NYC time:
			lastDate = toDate(now.Add(time.Duration(time.Hour * 24 * -7)).In(nyLoc))
		} else if err != nil {
			log.Println(err)
			continue
		} else {
			// Extract the last-fetched date from the db record in NYC time:
			lastDate, _ = time.ParseInLocation(time.RFC3339, lastDateRec.ClosingDate, nyLoc)
			lastDate = toDate(lastDate)
		}
		//fmt.Println(yesterday.Format("2006-01-02 15:04:05 -0700"))
		//fmt.Println(lastDate.Format("2006-01-02 15:04:05 -0700"))

		// Fetch the last few days' worth of historical data if we need to:
		fmt.Printf("  Yesterday's date:  %s\n", lastDate.Format(dateFmt))
		fmt.Printf("  Last date fetched: %s\n", lastDate.Format(dateFmt))
		if lastDate.Before(yesterday) {
			// TODO(jsd): YQL parameter escaping!

			// Fetch the last few days' worth of data:
			fmt.Printf("  Fetching historical data since %s...\n", lastDate.Format(dateFmt))
			hist := new(HistoricalResponse)
			if err = yql(hist, `select Date, Close from yahoo.finance.historicaldata where symbol = "`+st.Symbol+`" and startDate = "`+lastDate.Format(dateFmt)+`" and endDate = "`+yesterday.Format(dateFmt)+`"`); err != nil {
				log.Println(err)
				continue
			}
			fmt.Printf("  Fetched.\n")

			// Insert historical records:
			fmt.Printf("  Recording...\n")
			for _, value := range hist.Query.Results.Quote {
				// Store dates as RFC3339 in the NYC timezone:
				date, err := time.ParseInLocation(dateFmt, value.Date, nyLoc)
				if err != nil {
					log.Println(err)
					continue
				}

				// Insert the history record:
				db.Execl(`insert into stock_history (symbol, closing_date, closing_price) values (?,?,?)`, st.Symbol, date.Format(time.RFC3339), value.Close)
			}
			fmt.Printf("  Recorded.\n")
		}

		// Get the current stock price:
		fmt.Printf("  Fetching current trading price...\n")
		quot := new(QuoteResponse)
		err = yql(quot, `select LastTradePriceOnly from yahoo.finance.quote where symbol = "`+st.Symbol+`"`)
		if err != nil {
			log.Println(err)
			continue
		}

		// Work with prices in `big.Rat` types (arbitrary precision rational numbers):
		currPrice := toRat(quot.Query.Results.Quote.LastTradePriceOnly)
		fmt.Printf("  Fetched current trading price: %s\n", currPrice.FloatString(2))

		yesterdayPrice := toRat(lastDateRec.ClosingPrice)
		fmt.Printf("  Yesterday's close price:       %s\n", yesterdayPrice.FloatString(2))

		stopPrice := new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Sub(toRat("100"), toRat(st.TrailingStopPercent)), toRat("0.01"))), yesterdayPrice)
		fmt.Printf("  Stopping price:                %s\n", stopPrice.FloatString(2))
		if currPrice.Cmp(stopPrice) <= 0 {
			// Current price has fallen below stopping price!
			fmt.Println("  FAIL!")
		}
	}

	return
}
