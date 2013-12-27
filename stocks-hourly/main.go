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
	"math/big"
	"net/mail"
	"time"
)

// Our own packages:
import (
	"github.com/JamesDunne/StockWatcher/mailutil"
	"github.com/JamesDunne/StockWatcher/stocksAPI"
)

func currency(v *big.Rat) string {
	if v.Sign() > 0 {
		return "+" + v.FloatString(2)
	} else if v.Sign() == 0 {
		return " " + v.FloatString(2)
	} else {
		// Includes - sign:
		return v.FloatString(2)
	}
}

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
			// Real data from market:
			api.AddOwnedStock(testUser.UserID, "MSFT", "2013-09-03", stocksAPI.ToRat("31.88"), 10, stocksAPI.ToRat("2.50"))
			api.AddOwnedStock(testUser.UserID, "AAPL", "2013-09-03", stocksAPI.ToRat("488.58"), 10, stocksAPI.ToRat("2.50"))

			api.AddOwnedStock(testUser.UserID, "MSFT", "2013-09-03", stocksAPI.ToRat("31.88"), -5, stocksAPI.ToRat("2.50"))
			api.AddOwnedStock(testUser.UserID, "AAPL", "2013-09-03", stocksAPI.ToRat("488.58"), -5, stocksAPI.ToRat("2.50"))
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
		// Record trading history:
		log.Printf("  %s: recording historical data...\n", symbol)
		err = api.RecordHistory(symbol)
		if err != nil {
			log.Println(err)
			continue
		}

		// Calculate and record statistics:
		log.Printf("  %s: calculating statistics...\n", symbol)
		err = api.RecordStats(symbol)
		if err != nil {
			log.Println(err)
			continue
		}
	}

	// Fetch current prices from Yahoo into the database:
	log.Printf("Fetching current prices...\n")
	api.GetCurrentHourlyPrices(symbols...)

	for _, symbol := range symbols {
		// Calculate details of owned stocks and their owners for this symbol:
		owned, err := api.GetOwnedDetailsForSymbol(symbol)
		if err != nil {
			panic(err)
		}

		for _, own := range owned {
			// Get the owner:
			user, err := api.GetUser(own.UserID)
			if err != nil {
				panic(err)
			}

			log.Printf("  %s\n", symbol)
			log.Printf("    %s bought %d shares at %s on %s:\n", user.Name, own.Shares, own.BuyPrice.FloatString(2), own.BuyDate.Format(dateFmt))
			log.Printf("    current: %s\n", own.CurrPrice.FloatString(2))
			log.Printf("    t-stop:  %s\n", own.TStopPrice.FloatString(2))
			log.Printf("    gain($): %s\n", currency(own.GainLossDollar))
			log.Printf("    gain(%%): %.2f\n", own.GainLossPercent)

			// Check if (price < t-stop):
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
					body := fmt.Sprintf(`<html><body>%s current price %s fell below t-stop price %s</body></html>`, symbol, own.CurrPrice.FloatString(2), own.TStopPrice.FloatString(2))

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

			// TODO: check hard buy stop and sell stops.
		}
	}

	return
}
