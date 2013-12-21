package main

import "fmt"
import "log"
import _ "errors"
import "os"

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
	closing_date TEXT NOT NULL,
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

// main:
func main() {
	const dbPath = "./stocks.db"

	// Get the New York location for stock timezone:
	nyLoc, _ := time.LoadLocation("America/New_York")

	// Create our DB file and its schema:
	os.Remove(dbPath)
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
from stock_track as s
`); err != nil {
		log.Fatal(err)
	}

	fmt.Print("Stocks:\n")
	for _, st := range stocks {
		fmt.Printf("  %#v\n", st)

		// Determine the last-fetched date for the stock:
		rec := new(dbStockHistory)
		err = db.Get(rec, `
select h.symbol, h.closing_date, h.closing_price
from stock_history as h
where date(h.closing_date) = (select max(date(closing_date)) from stock_history where symbol = h.symbol)
  and h.symbol = ?
`, st.Symbol)

		var lastDate time.Time
		if err == sql.ErrNoRows {
			// No row; use 7 days ago in NYC time:
			lastDate = toDate(time.Now().Add(time.Duration(time.Hour * 24 * -7)).In(nyLoc))
		} else if err != nil {
			log.Fatal(err)
		} else {
			// Extract the last-fetched date from the db record in NYC time:
			lastDate, _ = time.ParseInLocation(time.RFC3339, rec.ClosingDate, nyLoc)
			lastDate = toDate(lastDate)
		}
		fmt.Println(lastDate.Format("2006-01-02 15:04:05 -0700"))

	}
	fmt.Println()

	//// get historical data for MSFT:
	//hist := new(HistoricalResponse)
	//if err = yql(hist, `select Date, Close from yahoo.finance.historicaldata where symbol = "MSFT" and startDate = "2013-12-16" and endDate = "2013-12-20"`); err != nil {
	//	log.Fatal(err)
	//	return
	//}

	//for i, value := range hist.Query.Results.Quote {
	//	if i == 0 {
	//		fmt.Printf("%#v", value)
	//	} else {
	//		fmt.Printf(", %#v", value)
	//	}
	//}
	//fmt.Println()

	//// get current price of MSFT:
	//quot := new(QuoteResponse)
	//err = yql(quot, `select Symbol, LastTradePriceOnly from yahoo.finance.quote where symbol in ("MSFT")`)
	//if err != nil {
	//	log.Fatal(err)
	//	return
	//}

	//fmt.Printf("%#v\n", quot.Query)

	return
}
