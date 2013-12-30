// ui.go
package main

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	//"net/url"
	//"path"
	"strconv"
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
	//"github.com/JamesDunne/StockWatcher/dbutil"
	//"github.com/JamesDunne/StockWatcher/mailutil"
	"github.com/JamesDunne/StockWatcher/stocks"
)

// utilities:

func getDetails(api *stocks.API, userID stocks.UserID) []stocks.StockDetail {
	details, err := api.GetStockDetailsForUser(userID)
	if err != nil {
		panic(err)
	}
	return details
}

func fetchLatest(api *stocks.API, symbols ...string) {
	// Run through each actively tracked stock and calculate stopping prices, notify next of kin, what have you...
	log.Printf("%d stocks tracked.\n", len(symbols))

	for _, symbol := range symbols {
		// Record trading history:
		log.Printf("%s: recording historical data...\n", symbol)
		err := api.RecordHistory(symbol)
		if err != nil {
			panic(err)
		}

		// Calculate and record statistics:
		log.Printf("%s: calculating statistics...\n", symbol)
		err = api.RecordStats(symbol)
		if err != nil {
			panic(err)
		}
	}

	// Fetch current prices from Yahoo into the database:
	log.Printf("Fetching current prices...\n")
	api.GetCurrentHourlyPrices(symbols...)
}

func notEmpty(s string, err string) string {
	if s == "" {
		panic(fmt.Errorf("Symbol required"))
	}
	return s
}

type BadRequestError struct {
	Message string
}

func (err BadRequestError) Error() string {
	return err.Message
}

func badRequest(err error, msg string) {
	if err != nil {
		log.Println(err)
		panic(BadRequestError{Message: msg})
	}
	return
}

func tryParseInt(s string, msg string) int64 {
	v, err := strconv.ParseInt(s, 10, 0)
	badRequest(err, msg)
	return v
}

// ----------------------- Secured section:

const dateFmt = "2006-01-02"

var uiTmpl *template.Template

// Handles /ui/* requests to present HTML UI to the user:
func uiHandler(w http.ResponseWriter, r *http.Request) {
	// Get API ready:
	api, err := stocks.NewAPI(dbPath)
	if err != nil {
		log.Println(err)
		http.Error(w, "Could not open stocks database!", http.StatusInternalServerError)
		return
	}
	defer api.Close()

	// Handle panic()s as '500' responses:
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			if bad, ok := err.(BadRequestError); ok {
				http.Error(w, bad.Error(), http.StatusBadRequest)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Normal execution.
	}()

	// Find user:
	webuser := getUserData(r)
	apiuser, err := api.GetUserByEmail(webuser.Email)
	if apiuser == nil || err != nil {
		if r.URL.Path != "/register" {
			http.Redirect(w, r, "/ui/register", http.StatusFound)
			return
		}
	}

	// Handle request:
	switch r.URL.Path {
	case "/register":
		if r.Method == "GET" {
			// Data to be used by the template:
			model := struct {
				WebUser *UserCookieData
				User    *stocks.User
			}{
				WebUser: webuser,
				User:    apiuser,
			}

			uiTmpl.ExecuteTemplate(w, "register", model)
		} else {
			// Assume POST.

			// Assume apiuser == nil, implying webuser.Email not found in database.

			// Add user:
			apiuser = &stocks.User{
				Name:                webuser.FullName,
				NotificationTimeout: time.Hour * time.Duration(24),
				Emails: []stocks.UserEmail{
					stocks.UserEmail{
						Email:     webuser.Email,
						IsPrimary: true,
					},
				},
			}

			err = api.AddUser(apiuser)
			if err != nil {
				panic(err)
			}

			http.Redirect(w, r, "/ui/dash", http.StatusFound)
		}
		return

		// -------------------------------------------------

	case "/dash":
		// Fetch data to be used by the template:
		details := getDetails(api, apiuser.UserID)
		owned := make([]stocks.StockDetail, 0, len(details))
		watched := make([]stocks.StockDetail, 0, len(details))

		for _, s := range details {
			if s.Stock.IsWatched {
				watched = append(watched, s)
			} else {
				owned = append(owned, s)
			}
		}

		model := struct {
			User    *stocks.User
			Owned   []stocks.StockDetail
			Watched []stocks.StockDetail
		}{
			User:    apiuser,
			Owned:   owned,
			Watched: watched,
		}

		err := uiTmpl.ExecuteTemplate(w, "dash", model)
		if err != nil {
			panic(err)
		}
		return

	case "/fetch":
		// Fetch latest data:

		// Query stocks:
		symbols, err := api.GetAllTrackedSymbols()
		if err != nil {
			panic(err)
		}

		fetchLatest(api, symbols...)

		// Redirect to dashboard with updated data:
		http.Redirect(w, r, "/ui/dash", http.StatusFound)
		return

		// -------------------------------------------------

	case "/stock/remove":
		if r.Method == "POST" {
			// Parse form data:
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}

			stockIDint, err := strconv.ParseInt(r.PostForm.Get("id"), 10, 0)
			if err != nil {
				panic(err)
			}
			stockID := stocks.StockID(stockIDint)

			err = api.RemoveStock(stockID)
			if err != nil {
				panic(err)
			}

			w.WriteHeader(200)
		}
		return

	case "/owned/add":
		if r.Method == "GET" {
			// Data to be used by the template:
			model := struct {
				User *stocks.User
			}{
				User: apiuser,
			}

			err := uiTmpl.ExecuteTemplate(w, "owned/add", model)
			if err != nil {
				panic(err)
			}
		} else {
			// Assume POST.

			// Parse form data:
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}

			s := &stocks.Stock{
				UserID:       apiuser.UserID,
				Symbol:       notEmpty(r.PostForm.Get("symbol"), "Symbol required"),
				BuyDate:      stocks.ToDateTime(dateFmt, r.PostForm.Get("buyDate")),
				BuyPrice:     stocks.ToDecimal(r.PostForm.Get("buyPrice")),
				Shares:       tryParseInt(r.PostForm.Get("shares"), "Invalid shares value"),
				IsWatched:    false,
				TStopPercent: stocks.ToNullDecimal(r.PostForm.Get("stopPercent")),
				// TODO: more fields!
			}

			// Add the stock:
			err = api.AddStock(s)
			if err != nil {
				panic(err)
			}

			// Fetch latest data for new symbol:
			fetchLatest(api, s.Symbol)

			http.Redirect(w, r, "/ui/dash", http.StatusFound)
		}
		return

	case "/watched/add":
		if r.Method == "GET" {
			// Data to be used by the template:
			model := struct {
				User *stocks.User
			}{
				User: apiuser,
			}

			err := uiTmpl.ExecuteTemplate(w, "watched/add", model)
			if err != nil {
				panic(err)
			}
		} else {
			// Assume POST.

			// Parse form data:
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}

			s := &stocks.Stock{
				UserID:       apiuser.UserID,
				Symbol:       notEmpty(r.PostForm.Get("symbol"), "Symbol required"),
				BuyDate:      stocks.ToDateTime(dateFmt, r.PostForm.Get("buyDate")),
				BuyPrice:     stocks.ToDecimal(r.PostForm.Get("buyPrice")),
				Shares:       tryParseInt(r.PostForm.Get("shares"), "Invalid shares value"),
				IsWatched:    true,
				TStopPercent: stocks.ToNullDecimal(r.PostForm.Get("stopPercent")),
				// TODO: more fields!
			}

			// Add the stock:
			err = api.AddStock(s)
			if err != nil {
				panic(err)
			}

			// Fetch latest data for new symbol:
			fetchLatest(api, s.Symbol)

			http.Redirect(w, r, "/ui/dash", http.StatusFound)
		}
		return
	}

	http.NotFound(w, r)
	return
}
