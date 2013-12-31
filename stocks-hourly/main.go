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
	"bytes"
	"flag"
	//"fmt"
	"html/template"
	"log"
	"net/mail"
	"time"
)

// Our own packages:
import (
	"github.com/JamesDunne/StockWatcher/mailutil"
	"github.com/JamesDunne/StockWatcher/stocks"
)

var emailTemplate *template.Template

func textTemplateString(tmpl *template.Template, name string, obj interface{}) string {
	w := new(bytes.Buffer)
	err := tmpl.ExecuteTemplate(w, name, obj)
	if err != nil {
		panic(err)
	}
	return w.String()
}

func attemptEmailUser(api *stocks.API, user *stocks.User, sd *stocks.StockDetail, lastDeliveryTime *stocks.NullDateTime, templateName string) bool {
	// Determine next available delivery time:
	nextDeliveryTime := time.Now()
	if (*lastDeliveryTime).Valid {
		nextDeliveryTime = (*lastDeliveryTime).Value.Add(user.NotificationTimeout)
	}

	// Can we deliver?
	if (*lastDeliveryTime).Valid && !time.Now().After(nextDeliveryTime) {
		log.Printf("  Not delivering notification email due to anti-spam timeout; next delivery after %s\n", nextDeliveryTime.Format(time.RFC3339))
		return false
	}

	log.Printf("  Delivering notification email to %s <%s>...\n", user.Name, user.PrimaryEmail())

	// Format mail addresses:
	from := mail.Address{"stock-watcher-" + sd.Stock.Symbol, "stock.watcher." + sd.Stock.Symbol + "@bittwiddlers.org"}
	to := mail.Address{user.Name, user.PrimaryEmail()}

	// Execute email template to get subject and body:
	subject := textTemplateString(emailTemplate, templateName+"/subject", sd)
	body := textTemplateString(emailTemplate, templateName+"/body", sd)

	// Deliver email:
	if err := mailutil.SendHtmlMessage(from, to, subject, body); err != nil {
		log.Println(err)
		log.Printf("  Failed delivering notification email.\n")
		return false
	} else {
		log.Printf("  Delivered notification email.\n")

		// Successfully delivered email as far as we know; record last delivery date/time:
		*lastDeliveryTime = stocks.NullDateTime{Value: time.Now(), Valid: true}
		api.UpdateNotifyTimes(&sd.Stock)
		return true
	}
}

// Notifications:

// Trailing Stop
func checkTStop(api *stocks.API, user *stocks.User, sd *stocks.StockDetail) {
	if !sd.Stock.NotifyTStop || !sd.Stock.TStopPercent.Valid {
		return
	}
	if !sd.Detail.CurrPrice.Valid || !sd.Detail.TStopPrice.Valid {
		return
	}

	// Check if (price < t-stop):
	if sd.Detail.CurrPrice.Value.Cmp(sd.Detail.TStopPrice.Value) > 0 {
		return
	}

	attemptEmailUser(api, user, sd, &sd.Stock.LastTimeTStop, "tstop")
}

// Buy Stop
func checkBuyStop(api *stocks.API, user *stocks.User, sd *stocks.StockDetail) {
	if !sd.Stock.NotifyBuyStop || !sd.Stock.BuyStopPrice.Valid {
		return
	}
	if !sd.Detail.CurrPrice.Valid {
		return
	}

	// Check if (price < buy-stop):
	if sd.Detail.CurrPrice.Value.Cmp(sd.Stock.BuyStopPrice.Value) > 0 {
		return
	}

	attemptEmailUser(api, user, sd, &sd.Stock.LastTimeBuyStop, "buystop")
}

// Sell Stop
func checkSellStop(api *stocks.API, user *stocks.User, sd *stocks.StockDetail) {
	if !sd.Stock.NotifySellStop || !sd.Stock.SellStopPrice.Valid {
		return
	}
	if !sd.Detail.CurrPrice.Valid {
		return
	}

	// Check if (price > sell-stop):
	if sd.Detail.CurrPrice.Value.Cmp(sd.Stock.SellStopPrice.Value) < 0 {
		return
	}

	attemptEmailUser(api, user, sd, &sd.Stock.LastTimeSellStop, "sellstop")
}

func checkRise(api *stocks.API, user *stocks.User, sd *stocks.StockDetail) {
	if !sd.Stock.NotifyRise || !sd.Stock.RisePercent.Valid {
		return
	}
	if !sd.Detail.CurrPrice.Valid || !sd.Detail.N1ClosePrice.Valid {
		return
	}

	// chg% = ((CurrPrice / N1ClosePrice) - 1) * 100
	chg := ((stocks.RatToFloat(sd.Detail.CurrPrice.Value) / stocks.RatToFloat(sd.Detail.N1ClosePrice.Value)) - 1.0) * 100.0
	if chg < stocks.RatToFloat(sd.Stock.RisePercent.Value) {
		return
	}

	attemptEmailUser(api, user, sd, &sd.Stock.LastTimeRise, "rise")
}

func checkFall(api *stocks.API, user *stocks.User, sd *stocks.StockDetail) {
	if !sd.Stock.NotifyFall || !sd.Stock.FallPercent.Valid {
		return
	}
	if !sd.Detail.CurrPrice.Valid || !sd.Detail.N1ClosePrice.Valid {
		return
	}

	// chg% = ((CurrPrice / N1ClosePrice) - 1) * 100
	chg := ((stocks.RatToFloat(sd.Detail.CurrPrice.Value) / stocks.RatToFloat(sd.Detail.N1ClosePrice.Value)) - 1.0) * 100.0
	if chg > -stocks.RatToFloat(sd.Stock.FallPercent.Value) {
		return
	}

	attemptEmailUser(api, user, sd, &sd.Stock.LastTimeFall, "fall")
}

func checkBullBear(api *stocks.API, user *stocks.User, sd *stocks.StockDetail) {
	if !sd.Stock.NotifyBullBear {
		return
	}
	if !sd.Detail.N1SMAPercent.Valid || !sd.Detail.N2SMAPercent.Valid {
		return
	}

	// TODO: verify this logic.
	if sd.Detail.N2SMAPercent.Value < 0.0 && sd.Detail.N1SMAPercent.Value >= 0.0 {
		attemptEmailUser(api, user, sd, &sd.Stock.LastTimeBullBear, "bull")
	} else if sd.Detail.N2SMAPercent.Value >= 0.0 && sd.Detail.N1SMAPercent.Value < 0.0 {
		attemptEmailUser(api, user, sd, &sd.Stock.LastTimeBullBear, "bear")
	}
}

// ------------- main:

func main() {
	const dateFmt = "2006-01-02"

	// Define our commandline flags:
	dbPathArg := flag.String("db", "../stocks-web/stocks.db", "Path to stocks.db database")
	mailServerArg := flag.String("mail-server", "localhost:25", "Address of SMTP server to use for sending email")
	testArg := flag.Bool("test", false, "Add test data")
	tmplPathArg := flag.String("template", "./emails.tmpl", "Path to email template file")

	// Parse the flags and set values:
	flag.Parse()
	dbPath := *dbPathArg
	mailutil.Server = *mailServerArg
	tmplPath := *tmplPathArg

	// Parse email template file:
	emailTemplate = template.Must(template.New("email").ParseFiles(tmplPath))

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
				Shares:       10,
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
		log.Printf("  %s: recording historical data and calculating statistics...\n", symbol)
		api.RecordHistory(symbol)
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
			if !sd.Stock.IsWatched {
				log.Printf("    %s bought %d shares at %s on %s:\n", user.Name, s.Shares, s.BuyPrice, s.BuyDate.DateString())
			} else {
				log.Printf("    %s watching from %s on %s:\n", user.Name, s.BuyPrice, s.BuyDate.DateString())
			}
			if sd.Detail.CurrPrice.Valid {
				log.Printf("    current: %v\n", sd.Detail.CurrPrice)
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

			// Check notifications:
			log.Println()

			// Current price has fallen below trailing-stop price!
			log.Println("  Checking trailing stop...")
			checkTStop(api, user, &sd)

			log.Println("  Checking buy stop...")
			checkBuyStop(api, user, &sd)

			log.Println("  Checking sell stop...")
			checkSellStop(api, user, &sd)

			log.Println("  Checking rise by %...")
			checkRise(api, user, &sd)

			log.Println("  Checking fall by %...")
			checkFall(api, user, &sd)

			log.Println("  Checking SMA for bullish/bearish...")
			checkBullBear(api, user, &sd)
		}
	}

	log.Println("Job complete")

	return
}
