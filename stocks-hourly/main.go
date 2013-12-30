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
	"net/mail"
	"time"
)

// Our own packages:
import (
	"github.com/JamesDunne/StockWatcher/mailutil"
	"github.com/JamesDunne/StockWatcher/stocks"
)

// ------------- main:

func main() {
	const dateFmt = "2006-01-02"

	// Define our commandline flags:
	dbPathArg := flag.String("db", "../stocks-web/stocks.db", "Path to stocks.db database")
	mailServerArg := flag.String("mail-server", "localhost:25", "Address of SMTP server to use for sending email")
	testArg := flag.Bool("test", false, "Add test data")

	// Parse the flags and set values:
	flag.Parse()
	dbPath := *dbPathArg
	mailutil.Server = *mailServerArg

	// Create the API context which initializes the database:
	api, err := stocks.NewAPI(dbPath)
	if err != nil {
		log.Fatalln(err)
		return
	}
	defer api.Close()

	// Testing data:
	if *testArg {
		testUser := &stocks.User{
			Name:                "Test User",
			NotificationTimeout: time.Minute,
			Emails: []stocks.UserEmail{
				stocks.UserEmail{Email: "test@example.org", IsPrimary: true},
			},
		}
		err := api.AddUser(testUser)

		if err == nil {
			// Real data from market:
			s := &stocks.Stock{
				UserID:       testUser.UserID,
				Symbol:       "MSFT",
				BuyDate:      stocks.ToDateTime(dateFmt, "2013-09-03"),
				BuyPrice:     stocks.ToDecimal("31.88"),
				Shares:       10,
				TStopPercent: stocks.ToNullDecimal("2.50"),
			}
			api.AddStock(s)
			s = &stocks.Stock{
				UserID:       testUser.UserID,
				Symbol:       "MSFT",
				BuyDate:      stocks.ToDateTime(dateFmt, "2013-09-03"),
				BuyPrice:     stocks.ToDecimal("31.88"),
				Shares:       -5,
				TStopPercent: stocks.ToNullDecimal("2.50"),
			}
			api.AddStock(s)

			s = &stocks.Stock{
				UserID:       testUser.UserID,
				Symbol:       "AAPL",
				BuyDate:      stocks.ToDateTime(dateFmt, "2013-09-03"),
				BuyPrice:     stocks.ToDecimal("488.58"),
				Shares:       -5,
				TStopPercent: stocks.ToNullDecimal("2.50"),
			}
			api.AddStock(s)
			s = &stocks.Stock{
				UserID:       testUser.UserID,
				Symbol:       "AAPL",
				BuyDate:      stocks.ToDateTime(dateFmt, "2013-09-03"),
				BuyPrice:     stocks.ToDecimal("488.58"),
				Shares:       10,
				TStopPercent: stocks.ToNullDecimal("2.50"),
			}
			api.AddStock(s)

			s = &stocks.Stock{
				UserID:       testUser.UserID,
				Symbol:       "YHOO",
				BuyDate:      stocks.ToDateTime(dateFmt, "2013-09-03"),
				BuyPrice:     stocks.ToDecimal("31.88"),
				Shares:       0,
				IsWatched:    true,
				TStopPercent: stocks.ToNullDecimal("2.50"),
			}
			api.AddStock(s)
		}
	}

	// Get today's date in NY time:
	//today, lastTradeDate := api.Today(), api.LastTradingDate()
	//if stocks.IsWeekend(today) {
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
		details, err := api.GetStockDetailsForSymbol(symbol)
		if err != nil {
			panic(err)
		}

		for _, sd := range details {
			s := &sd.Stock
			d := &sd.Detail

			// Get the owner:
			user, err := api.GetUser(s.UserID)
			if err != nil {
				panic(err)
			}

			log.Printf("  %s\n", symbol)
			log.Printf("    %s bought %d shares at %s on %s:\n", user.Name, s.Shares, s.BuyPrice, s.BuyDate.DateString())
			if sd.CurrPrice.Valid {
				log.Printf("    current: %v\n", sd.CurrPrice)
			}
			if d.TStopPrice.Valid {
				log.Printf("    t-stop:  %v\n", d.TStopPrice)
			}
			if d.GainLossDollar.Valid {
				log.Printf("    gain($): %v\n", d.GainLossDollar.CurrencyString())
			}
			if d.GainLossPercent.Valid {
				log.Printf("    gain(%%): %v\n", d.GainLossPercent)
			}

			// Check if (price < t-stop):
			if s.NotifyTStop && sd.CurrPrice.Valid && d.TStopPrice.Valid && sd.CurrPrice.Value.Cmp(d.TStopPrice.Value) <= 0 {
				// Current price has fallen below trailing-stop price!
				log.Println()
				log.Println("  ALERT: Current price has fallen below trailing-stop price!")

				// Determine next available delivery time:
				nextDeliveryTime := time.Now()
				if s.LastTimeTStop.Valid {
					nextDeliveryTime = s.LastTimeTStop.Value.Add(user.NotificationTimeout)
				}

				// Can we deliver?
				if !s.LastTimeTStop.Valid || time.Now().After(nextDeliveryTime) {
					log.Printf("  Delivering notification email to %s <%s>...\n", user.Name, user.PrimaryEmail())

					// Format mail addresses:
					from := mail.Address{"stock-watcher-" + symbol, "stock.watcher." + symbol + "@bittwiddlers.org"}
					to := mail.Address{user.Name, user.PrimaryEmail()}

					// Format subject and body:
					subject := fmt.Sprintf("%s price fell below %s", symbol, d.TStopPrice)
					body := fmt.Sprintf(`<html><body>%s current price %s fell below t-stop price %s</body></html>`, symbol, sd.CurrPrice, d.TStopPrice)

					// Deliver email:
					if err = mailutil.SendHtmlMessage(from, to, subject, body); err != nil {
						log.Println(err)
						log.Printf("  Failed delivering notification email.\n")
					} else {
						log.Printf("  Delivered notification email.\n")

						// Successfully delivered email as far as we know; record last delivery date/time:
						s.LastTimeTStop = stocks.NullDateTime{Value: time.Now(), Valid: true}
						api.UpdateNotifyTimes(s)
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
