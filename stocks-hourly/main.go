/*	StockWatcher
	James Dunne
	https://github.com/JamesDunne/StockWatcher

	This program is an hourly cron job to watch a set of stocks and notify the
	owner via email if the price drops below (100 - N) percent of the highest
	historical closing price.
*/
package main

// general stuff:
import (
	"flag"
	"fmt"
	"log"
	//"math/big"
	"net/mail"
	"time"
)

// sqlite related imports:
import (
	//"database/sql"
	//"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Our own packages:
import (
	"github.com/JamesDunne/StockWatcher/mailutil"
	"github.com/JamesDunne/StockWatcher/stocksAPI"
)

// ------------- main:

func main() {
	const dateFmt = "2006-01-02"

	// Define our commandline flags:
	dbPathArg := flag.String("db", "../stocks-web/stocks.db", "Path to stocks.db database")
	mailServerArg := flag.String("mail-server", "localhost:25", "Address of SMTP server to use for sending email")

	// Parse the flags and set values:
	flag.Parse()
	dbPath := *dbPathArg
	mailutil.Server = *mailServerArg

	// Create the API context which initializes the database:
	api, err := stocksAPI.NewAPI(dbPath)
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer api.Close()

	// Testing data:
	{
		testUser := &stocksAPI.User{
			PrimaryEmail:        "test@example.org",
			Name:                "Test User",
			NotificationTimeout: time.Minute,
		}
		err := api.AddUser(testUser)

		if err == nil {
			api.AddOwnedStock(testUser.UserID, "MSFT", "2013-09-01", stocksAPI.ToRat("30.00"), 10, stocksAPI.ToRat("25.00"))
		}
	}

	// Get today's date in NY time:
	//today, lastTradeDate := api.Today(), api.LastTradingDate()
	//if stocksAPI.IsWeekend(today) {
	//	// We don't work on weekends.
	//	log.Printf("No work to do on weekends.")
	//	return
	//}

	// Query stocks:
	symbols, err := api.GetAllTrackedSymbols()
	if err != nil {
		log.Fatalln(err)
		return
	}

	// Run through each actively tracked stock and calculate stopping prices, notify next of kin, what have you...
	log.Printf("%d stocks tracked.\n", len(symbols))

	for _, symbol := range symbols {
		log.Printf("%s:\n", symbol)

		// Record trading history:
		err = api.RecordHistory(symbol)
		if err != nil {
			log.Println(err)
			continue
		}

		// Calculate and record statistics:
		err = api.RecordStats(symbol)
		if err != nil {
			log.Println(err)
			continue
		}
	}

	// Fetch hourly prices into the database:
	api.GetCurrentHourlyPrices(symbols...)

	for _, symbol := range symbols {
		// Find details of owned stocks and their owners for this symbol:
		owned, err := api.GetOwnedDetailsForSymbol(symbol)
		if err != nil {
			panic(err)
		}
		log.Printf("  %+v\n", owned)

		for _, own := range owned {
			user, err := api.GetUser(own.UserID)
			if err != nil {
				panic(err)
			}

			if own.CurrPrice.Cmp(own.TStopPrice) <= 0 {
				// Current price has fallen below trailing-stop price!
				log.Println()
				log.Println("  ALERT: Current price has fallen below trailing-stop price!")

				// Determine next available delivery time:
				nextDeliveryTime := time.Now()
				if own.LastTStopNotifyTime != nil {
					nextDeliveryTime = (*own.LastTStopNotifyTime).Add(user.NotificationTimeout)
				}

				// Can we deliver?
				if own.LastTStopNotifyTime == nil || time.Now().After(nextDeliveryTime) {
					log.Printf("  Delivering notification email to %s <%s>...\n", user.Name, user.PrimaryEmail)

					// Format mail addresses:
					from := mail.Address{"stock-watcher-" + symbol, "stock.watcher." + symbol + "@bittwiddlers.org"}
					to := mail.Address{user.Name, user.PrimaryEmail}

					// Format subject and body:
					subject := symbol + " price fell below " + own.TStopPrice.FloatString(2)
					body := fmt.Sprintf(`<html><body>%s current price %s just fell below stop price %s</body></html>`, symbol, own.CurrPrice.FloatString(2), own.TStopPrice.FloatString(2))

					// Deliver email:
					if err = mailutil.SendHtmlMessage(from, to, subject, body); err != nil {
						log.Println(err)
						log.Printf("  Failed delivering notification email.\n")
					} else {
						log.Printf("  Delivered notification email.\n")
						// Successfully delivered email as far as we know; record last delivery date/time:
						api.UpdateOwnedLastNotifyTime(own.OwnedID, time.Now())
					}
				} else {
					log.Printf("  Not delivering notification email due to anti-spam timeout; next delivery after %s\n", nextDeliveryTime.Format(time.RFC3339))
				}
			}
		}
	}

	//for _, st := range trackedSymbols {
	//	// NOTE(jsd): Stock dates/times are in NYC timezone.
	//	purchaseDate, err := time.Parse(dateFmt, st.PurchaseDate)
	//	if err != nil {
	//		log.Printf("Error parsing PurchaseDate for '%s': %s\n", st.Symbol, err)
	//		continue
	//	}

	//	log.Printf("'%s' purchased by %s <%s> on %s for %s with %s%% trailing stop\n", st.Symbol, st.UserName, st.UserEmail, purchaseDate.Format(dateFmt), stocksAPI.ToRat(st.PurchasePrice).FloatString(2), stocksAPI.ToRat(st.StopPercent).FloatString(2))

	//	// Start fetching the current stock price from Yahoo! Finance:
	//	taskCurrPrice := make(chan *big.Rat)
	//	go func(query string) {
	//		log.Printf("  Fetching current trading price...\n")

	//		quot := make([]yqlQuote, 0, 1)
	//		err := yql.Get(&quot, query)
	//		if err != nil {
	//			log.Println(err)
	//			taskCurrPrice <- nil
	//			return
	//		}
	//		log.Printf("  Fetched current trading price.\n")

	//		taskCurrPrice <- stocksAPI.ToRat(quot[0].LastTradePriceOnly)
	//	}(`select LastTradePriceOnly from yahoo.finance.quote where symbol = "` + st.Symbol + `"`)

	//	// Determine the highest and lowest closing price from historical data:
	//	maxmin, err := dbutil.GetScalars(db, `select max(cast(Closing as real)), min(cast(Closing as real)) from StockHistory where Symbol = ?1`, st.Symbol)
	//	if err == sql.ErrNoRows {
	//		maxmin = []interface{}{"", ""}
	//	} else if err != nil {
	//		log.Println(err)
	//		continue
	//	}
	//	if maxmin[0] == nil {
	//		maxmin[0] = "0"
	//	}
	//	if maxmin[1] == nil {
	//		maxmin[1] = "0"
	//	}
	//	highestPrice, lowestPrice := stocksAPI.ToRat(maxmin[0].(string)), stocksAPI.ToRat(maxmin[1].(string))

	//	// stopPrice = ((100 - stopPercent) * 0.01) * highestPrice
	//	stopPrice := new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Sub(stocksAPI.ToRat("100"), stocksAPI.ToRat(st.StopPercent)), stocksAPI.ToRat("0.01"))), highestPrice)

	//	// Calculate long running averages:
	//	avgs, err := dbutil.GetScalars(db, `
	//select (select avg(cast(Closing as real)) from StockHistory where Symbol = ?1 and (datetime(Date) between datetime(?2, '-50 days') and datetime(?2))) as avg50,
	//      (select avg(cast(Closing as real)) from StockHistory where Symbol = ?1 and (datetime(Date) between datetime(?2,'-200 days') and datetime(?2))) as avg200`,
	//		st.Symbol,
	//		today.Format(time.RFC3339),
	//	)
	//	if err == sql.ErrNoRows {
	//		avgs = []interface{}{"", ""}
	//	} else if err != nil {
	//		log.Println(err)
	//		continue
	//	}
	//	if avgs[0] == nil {
	//		avgs[0] = "0"
	//	}
	//	if avgs[1] == nil {
	//		avgs[1] = "0"
	//	}
	//	avg50, avg200 := stocksAPI.ToRat(avgs[0].(string)), stocksAPI.ToRat(avgs[1].(string))

	//	log.Println()
	//	log.Printf("  Highest closing price:  %s\n", highestPrice.FloatString(2))
	//	log.Printf("  Lowest closing price:   %s\n", lowestPrice.FloatString(2))
	//	log.Printf("  50-day moving average:  %s\n", avg50.FloatString(2))
	//	log.Printf("  200-day moving average: %s\n", avg200.FloatString(2))
	//	log.Println()

	//	// Wait for current price data to come back:
	//	currPrice := <-taskCurrPrice
	//	if currPrice == nil {
	//		log.Printf("  Error while fetching current trading price.\n")
	//		continue
	//	}

	//	log.Printf("  Current price:  %s\n", currPrice.FloatString(2))
	//	log.Printf("  Stopping price: %s\n", stopPrice.FloatString(2))

	//	if currPrice.Cmp(stopPrice) <= 0 {
	//		// Current price has fallen below stopping price!
	//		log.Println()
	//		log.Println("  ALERT: Current price has fallen below stop price!")

	//		// Check DB to see if notification already sent:
	//		var nextDeliveryTime time.Time
	//		if st.StopLastNotificationDate.Valid {
	//			nextDeliveryTime, err = toDateTime(st.StopLastNotificationDate.String, nil)
	//			if err != nil {
	//				log.Printf("  Error parsing StopLastNotificationDate: %s\n", err)
	//				continue
	//			}
	//			nextDeliveryTime = nextDeliveryTime.Add(time.Duration(st.UserNotificationTimeout) * time.Second)
	//		}

	//		if !st.StopLastNotificationDate.Valid || time.Now().After(nextDeliveryTime) {
	//			log.Printf("  Delivering notification email to %s <%s>...\n", st.UserName, st.UserEmail)

	//			// Format mail addresses:
	//			from := mail.Address{"stock-watcher-" + st.Symbol, "stock.watcher." + st.Symbol + "@bittwiddlers.org"}
	//			to := mail.Address{st.UserName, st.UserEmail}

	//			// Format subject and body:
	//			subject := st.Symbol + " price fell below " + stopPrice.FloatString(2)
	//			body := fmt.Sprintf(`<html><body>%s current price %s just fell below stop price %s</body></html>`, st.Symbol, currPrice.FloatString(2), stopPrice.FloatString(2))

	//			// Deliver email:
	//			if err = mailutil.SendHtmlMessage(from, to, subject, body); err != nil {
	//				log.Println(err)
	//				log.Printf("  Failed delivering notification email.\n")
	//			} else {
	//				log.Printf("  Delivered notification email.\n")
	//				// Successfully delivered email as far as we know; record last delivery date/time:
	//				db.Execl(
	//					`update StockOwned set StopLastNotificationDate = ?1 where rowid = ?2`,
	//					time.Now().Format(time.RFC3339),
	//					st.StockOwnedID,
	//				)
	//			}
	//		} else {
	//			log.Printf("  Not delivering notification email due to anti-spam timeout; next delivery after %s\n", nextDeliveryTime.Format(time.RFC3339))
	//		}
	//	}
	//}

	return
}
