/*	StockWatcher
	James Dunne
	https://github.com/JamesDunne/StockWatcher

	This program is an hourly cron job to watch a set of stocks and notify the
	owner via email if the price drops below (100 - N) percent of the highest
	historical closing price.
*/
package main

// general stuff:
import "fmt"
import "log"
import "time"
import "math/big"

//import "os"

import "net/mail"

// sqlite related imports:
import "database/sql"
import "github.com/jmoiron/sqlx"
import _ "github.com/mattn/go-sqlite3"

import "github.com/JamesDunne/StockWatcher/stocksdb"
import "github.com/JamesDunne/StockWatcher/dbutil"
import "github.com/JamesDunne/StockWatcher/yql"
import "github.com/JamesDunne/StockWatcher/mailutil"

// ------------- Utility functions:

// Converts a string into a `*big.Rat` which is an arbitrary precision rational number stored in decimal format
func toRat(v string) *big.Rat {
	rat := new(big.Rat)
	rat.SetString(v)
	return rat
}

// ------------- data structures:

// Head to http://developer.yahoo.com/yql/console/?q=select%20*%20from%20yahoo.finance.quote%20where%20symbol%20in%20(%22YHOO%22%2C%22AAPL%22%2C%22GOOG%22%2C%22MSFT%22)&env=store%3A%2F%2Fdatatables.org%2Falltableswithkeys
// to understand this JSON structure.

type yqlQuote struct {
	Symbol             string
	LastTradePriceOnly string
}

type yqlHistory struct {
	Symbol string
	Date   string
	Open   string
	Close  string
	High   string
	Low    string
	Volume string
}

// from StockOwned joined with User:
type dbStock struct {
	UserID                  int    `db:"UserID"`
	UserEmail               string `db:"UserEmail"`
	UserName                string `db:"UserName"`
	UserNotificationTimeout int    `db:"UserNotificationTimeout"` // timeout in seconds

	StockOwnedID             int            `db:"StockOwnedID"`
	Symbol                   string         `db:"Symbol"`
	PurchasePrice            string         `db:"PurchasePrice"`
	PurchaseDate             string         `db:"PurchaseDate"`
	PurchaserEmail           string         `db:"PurchaserEmail"`
	StopPercent              string         `db:"StopPercent"`
	StopLastNotificationDate sql.NullString `db:"StopLastNotificationDate"`
}

// ------------- main:

func main() {
	const dbPath = "./stocks.db"
	const dateFmt = "2006-01-02"

	// Get the New York location for stock timezone:
	nyLoc, _ := time.LoadLocation("America/New_York")

	// Create our DB file and its schema:
	db, err := stocksdb.CreateSchema(dbPath)
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer db.Close()

	// Add some test data:
	dbutil.Tx(db, func(tx *sqlx.Tx) (err error) {
		db.Execl(`insert or ignore into User (Email, Name, NotificationTimeout) values ('example@example.org', 'Example User', 15)`)
		db.Execl(`insert or ignore into StockOwned (UserID, Symbol, IsStopEnabled, PurchaseDate, PurchasePrice, StopPercent) values (1, 'MSFT', 1, '2012-09-01', '30.00', '0.1');`)
		db.Execl(`insert or ignore into StockOwned (UserID, Symbol, IsStopEnabled, PurchaseDate, PurchasePrice, StopPercent) values (1, 'AAPL', 1, '2012-09-01', '400.00', '20.0');`)
		return nil
	})

	// Query stocks table:
	stocks := make([]dbStock, 0, 4) // make(type, len, capacity)
	if err = db.Select(&stocks, `
select s.UserID, u.Email as UserEmail, u.Name as UserName, u.NotificationTimeout AS UserNotificationTimeout
     , s.rowid as StockOwnedID, s.Symbol, s.PurchaseDate, s.PurchasePrice, s.StopPercent, s.StopLastNotificationDate
from StockOwned as s
join User as u on u.rowid = s.UserID
where s.IsStopEnabled = 1`); err != nil {
		log.Fatalln(err)
		return
	}

	// Get today's date in NY time:
	today := truncDate(time.Now().In(nyLoc))
	//if today.Weekday() == 0 || today.Weekday() == 6 {
	//	// We don't work on weekends.
	//	log.Printf("No work to do on weekends.")
	//	return
	//}

	// Find the last weekday trading date:
	// NOTE(jsd): Might screw up around DST changeover dates; who cares.
	yesterday := today.Add(time.Hour * time.Duration(-24))
	for yesterday.Weekday() == 0 || yesterday.Weekday() == 6 {
		yesterday = yesterday.Add(time.Hour * time.Duration(-24))
	}

	// Run through each actively tracked stock and calculate stopping prices, notify next of kin, what have you...
	log.Printf("%d stocks tracked.\n", len(stocks))
	for _, st := range stocks {
		// NOTE(jsd): Stock dates/times are in NYC timezone.
		purchaseDate, _ := time.Parse(dateFmt, st.PurchaseDate)

		log.Printf("'%s' purchased by %s <%s> on %s for %s with %s%% trailing stop\n", st.Symbol, st.UserName, st.UserEmail, purchaseDate.Format(dateFmt), toRat(st.PurchasePrice).FloatString(2), toRat(st.StopPercent).FloatString(2))

		// Determine the last-fetched date for the stock, assuming no holes exist in the dates:
		var lastDate time.Time
		ld, err := dbutil.GetScalar(db, `select h.Date from StockHistory h where (h.Symbol = ?1) and (datetime(h.Date) = (select max(datetime(Date)) from StockHistory where Symbol = h.Symbol))`, st.Symbol)
		if ld == nil {
			// No rows; fetch all the way back to purchase date
			lastDate = purchaseDate
		} else {
			// Extract the last-fetched date from the db record, assuming NYC time:
			lastDate = truncDate(toDateTime(ld.(string), nyLoc))
		}

		// Start fetching the current stock price from Yahoo! Finance:
		taskCurrPrice := make(chan *big.Rat)
		go func(query string) {
			log.Printf("  Fetching current trading price...\n")

			quot := make([]yqlQuote, 0, 1)
			err := yql.Get(&quot, query)
			if err != nil {
				log.Println(err)
				taskCurrPrice <- nil
				return
			}
			log.Printf("  Fetched current trading price.\n")

			taskCurrPrice <- toRat(quot[0].LastTradePriceOnly)
		}(`select LastTradePriceOnly from yahoo.finance.quote where symbol = "` + st.Symbol + `"`)

		log.Printf("  Last trading date: %s\n", yesterday.Format(dateFmt))
		log.Printf("  Last date fetched: %s\n", lastDate.Format(dateFmt))

		// Fetch the last few days' worth of historical data if we need to:
		if lastDate.Before(yesterday) {
			// Fetch the last few days' worth of data:
			log.Printf("  Fetching historical data since %s...\n", lastDate.Format(dateFmt))

			// TODO(jsd): YQL parameter escaping!
			yqlHistQuery := `select Symbol, Date, Open, Close, High, Low, Volume from yahoo.finance.historicaldata where symbol = "` + st.Symbol + `" and startDate = "` + lastDate.Format(dateFmt) + `" and endDate = "` + yesterday.Format(dateFmt) + `"`

			hist := make([]yqlHistory, 0, 10)
			if err = yql.Get(&hist, yqlHistQuery); err != nil {
				log.Println(err)
				continue
			}
			log.Printf("  Fetched historical data.\n")

			// Bulk insert the historical data into the StockHistory table:
			log.Printf("  Recording historical data...\n")

			rows := make([][]interface{}, 0, len(hist))
			for _, h := range hist {
				// Store dates as RFC3339 in the NYC timezone:
				date, err := time.ParseInLocation(dateFmt, h.Date, nyLoc)
				if err != nil {
					log.Println(err)
					continue
				}

				rows = append(rows, []interface{}{
					st.Symbol,
					date.Format(time.RFC3339),
					h.Close,
					h.Open,
					h.High,
					h.Low,
					h.Volume,
				})
			}

			err = dbutil.BulkInsert(db, "StockHistory", []string{"Symbol", "Date", "Closing", "Opening", "High", "Low", "Volume"}, rows)
			if err != nil {
				log.Println(err)
				continue
			}

			log.Printf("  Recorded historical data.\n")
		}

		// Determine the highest and lowest closing price from historical data:
		maxmin, err := dbutil.GetScalars(db, `select max(cast(Closing as real)), min(cast(Closing as real)) from StockHistory where Symbol = ?1`, st.Symbol)
		if err == sql.ErrNoRows {
			maxmin = []interface{}{"", ""}
		} else if err != nil {
			log.Println(err)
			continue
		}
		highestPrice, lowestPrice := toRat(maxmin[0].(string)), toRat(maxmin[1].(string))

		// stopPrice = ((100 - stopPercent) * 0.01) * highestPrice
		stopPrice := new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Sub(toRat("100"), toRat(st.StopPercent)), toRat("0.01"))), highestPrice)

		// Calculate long running averages:
		avgs, err := dbutil.GetScalars(db, `
select (select avg(cast(Closing as real)) from StockHistory where Symbol = ?1 and (datetime(Date) between datetime(?2, '-50 days') and datetime(?2))) as avg50,
       (select avg(cast(Closing as real)) from StockHistory where Symbol = ?1 and (datetime(Date) between datetime(?2,'-200 days') and datetime(?2))) as avg200`,
			st.Symbol,
			today.Format(time.RFC3339),
		)
		if err == sql.ErrNoRows {
			avgs = []interface{}{"", ""}
		} else if err != nil {
			log.Println(err)
			continue
		}
		avg50, avg200 := toRat(avgs[0].(string)), toRat(avgs[1].(string))

		log.Println()
		log.Printf("  Highest closing price:  %s\n", highestPrice.FloatString(2))
		log.Printf("  Lowest closing price:   %s\n", lowestPrice.FloatString(2))
		log.Printf("  50-day moving average:  %s\n", avg50.FloatString(2))
		log.Printf("  200-day moving average: %s\n", avg200.FloatString(2))
		log.Println()

		// Wait for current price data to come back:
		currPrice := <-taskCurrPrice
		if currPrice == nil {
			log.Printf("  Error while fetching current trading price.")
			continue
		}

		log.Printf("  Current price:  %s\n", currPrice.FloatString(2))
		log.Printf("  Stopping price: %s\n", stopPrice.FloatString(2))
		log.Println()

		if currPrice.Cmp(stopPrice) <= 0 {
			// Current price has fallen below stopping price!
			log.Println("  ALERT: Current price has fallen below stop price!")

			// Check DB to see if notification already sent:
			nextDeliveryTime := toDateTime(st.StopLastNotificationDate.String, nil).Add(time.Duration(st.UserNotificationTimeout) * time.Second)
			if !st.StopLastNotificationDate.Valid || time.Now().After(nextDeliveryTime) {
				log.Printf("  Delivering notification email to %s <%s>...\n", st.UserName, st.UserEmail)

				// Format mail addresses:
				from := mail.Address{"stock-watcher-" + st.Symbol, "stock.watcher." + st.Symbol + "@bittwiddlers.org"}
				to := mail.Address{st.UserName, st.UserEmail}

				// Format subject and body:
				subject := st.Symbol + " price fell below " + stopPrice.FloatString(2)
				body := fmt.Sprintf(`<html><body>%s current price %s just fell below stop price %s</body></html>`, st.Symbol, currPrice.FloatString(2), stopPrice.FloatString(2))

				// Deliver email:
				if err = mailutil.SendHtmlMessage(from, to, subject, body); err != nil {
					log.Println(err)
					log.Printf("  Failed delivering notification email.\n")
				} else {
					log.Printf("  Delivered notification email.\n")
					// Successfully delivered email as far as we know; record last delivery date/time:
					db.Execl(
						`update StockOwned set StopLastNotificationDate = ?1 where rowid = ?2`,
						time.Now().Format(time.RFC3339),
						st.StockOwnedID,
					)
				}
			} else {
				log.Printf("  Not delivering notification email due to anti-spam timeout; next delivery after %s\n", nextDeliveryTime.Format(time.RFC3339))
			}
		}
	}

	return
}
