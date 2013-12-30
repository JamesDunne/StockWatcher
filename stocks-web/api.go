// api.go
package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	//"net/url"
	//"time"
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

// JSON response wrapper:
type jsonResponse struct {
	Success bool             `json:"success"`
	Error   *json.RawMessage `json:"error"`
	Result  *json.RawMessage `json:"result"`
}

var null = json.RawMessage([]byte("null"))

func panicIf(err error) {
	if err != nil {
		panic(err)
	}
}

func parsePostJson(r *http.Request, data interface{}) {
	postjson, err := ioutil.ReadAll(r.Body)
	panicIf(err)

	err = json.Unmarshal(postjson, data)
	panicIf(err)
}

func validate(test bool, msg string) {
	if !test {
		panic(BadRequestError{Message: msg})
	}
}

// Handles /api/* requests for JSON API:
func apiHandler(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user data:
	webuser := getUserData(r)

	// Set these values in the API switch handler below:
	var rspcode int = 200
	var rsperr error
	var rsp interface{}

	// Send JSON response at end:
	defer func() {
		// Determine if error or success:
		if err := recover(); err != nil {
			if bad, ok := err.(BadRequestError); ok {
				rsperr = bad
				rspcode = 400
			} else {
				log.Println(err)
				rsperr = fmt.Errorf("Internal server error")
			}
		}

		var jrsp jsonResponse
		if rsperr != nil {
			// Error response:
			bytes, err := json.Marshal(rsperr.Error())
			if err != nil {
				log.Println(err)
				http.Error(w, "Error marshaling JSON error respnose", http.StatusInternalServerError)
				return
			}
			jrsp = jsonResponse{
				Success: false,
				Error:   new(json.RawMessage),
				Result:  &null,
			}
			*jrsp.Error = json.RawMessage(bytes)

			// Default to 500 error if unspecified:
			if rspcode == 200 {
				rspcode = 500
			}
		} else {
			// Success response:
			bytes, err := json.Marshal(rsp)
			if err != nil {
				log.Println(err)
				http.Error(w, "Error marshaling JSON success respnose", http.StatusInternalServerError)
				return
			}
			jrsp = jsonResponse{
				Success: true,
				Error:   &null,
				Result:  new(json.RawMessage),
			}
			*jrsp.Result = json.RawMessage(bytes)
		}

		// Marshal the root JSON response structure to a []byte:
		bytes, err := json.Marshal(jrsp)
		if err != nil {
			log.Println(err)
			http.Error(w, "Error marshaling root JSON respnose", http.StatusInternalServerError)
			return
		}

		// Send the JSON response:
		w.Header().Set("Content-Type", `application/json; charset="utf-8"`)
		w.WriteHeader(rspcode)

		w.Write(bytes)
	}()

	// Open API database:
	api, err := stocks.NewAPI(dbPath)
	if err != nil {
		log.Println(err)
		http.Error(w, "Could not open stocks database!", http.StatusInternalServerError)
		return
	}
	defer api.Close()

	// Get API user:
	apiuser, err := api.GetUserByEmail(webuser.Email)
	if err != nil {
		log.Println(err)
	}

	// Handle API urls:
	if r.Method == "GET" {
		// GET

		switch r.URL.Path {
		case "/user/who":
			rsp = apiuser

		case "/owned/list":
			// Get list of owned stocks w/ details.
			owned, _ := getDetailsSplit(api, apiuser.UserID)
			rsp = owned

		case "/watched/list":
			// Get list of watched stocks w/ details.
			_, watched := getDetailsSplit(api, apiuser.UserID)
			rsp = watched

		default:
			rspcode = 404
			rsperr = fmt.Errorf("Invalid API url")
		}
	} else {
		// POST

		switch r.URL.Path {
		case "/user/register":
			// Register new user with primary email.
			rsperr = fmt.Errorf("TODO")

		case "/user/join":
			// Join to existing user with secondary email.
			rsperr = fmt.Errorf("TODO")

		case "/stock/add":
			// Add stock.

			// Parse body as JSON:
			tmp := struct {
				Symbol    string
				BuyDate   string
				BuyPrice  string
				Shares    int64
				IsWatched bool

				TStopPercent   string
				BuyStopPrice   string
				SellStopPrice  string
				RisePercent    string
				FallPercent    string
				NotifyBullBear bool
			}{}
			parsePostJson(r, &tmp)

			// Validate settings and respond 400 if failed:
			validate(tmp.Symbol != "", "Symbol required")
			validate(tmp.BuyDate != "", "BuyDate required")
			validate(tmp.BuyPrice != "", "BuyPrice required")

			// Convert JSON input into stock struct:
			s := &stocks.Stock{
				UserID:    apiuser.UserID,
				Symbol:    tmp.Symbol,
				BuyDate:   stocks.ToDateTime(dateFmt, tmp.BuyDate),
				BuyPrice:  stocks.ToDecimal(tmp.BuyPrice),
				Shares:    tmp.Shares,
				IsWatched: tmp.IsWatched,

				TStopPercent:   stocks.ToNullDecimal(tmp.TStopPercent),
				BuyStopPrice:   stocks.ToNullDecimal(tmp.BuyStopPrice),
				SellStopPrice:  stocks.ToNullDecimal(tmp.SellStopPrice),
				RisePercent:    stocks.ToNullDecimal(tmp.RisePercent),
				FallPercent:    stocks.ToNullDecimal(tmp.FallPercent),
				NotifyBullBear: tmp.NotifyBullBear,
			}

			// Enable/disable notifications based on what's filled out:
			if s.TStopPercent.Valid {
				s.NotifyTStop = true
			}
			if s.BuyStopPrice.Valid {
				s.NotifyBuyStop = true
			}
			if s.SellStopPrice.Valid {
				s.NotifySellStop = true
			}
			if s.RisePercent.Valid {
				s.NotifyRise = true
			}
			if s.FallPercent.Valid {
				s.NotifyFall = true
			}

			// Add the stock record:
			err = api.AddStock(s)
			panicIf(err)

			// Check if we have to recreate history:
			minBuyDate := api.GetMinBuyDate(s.Symbol)
			if minBuyDate.Valid && s.BuyDate.Value.Before(minBuyDate.Value) {
				// Introducing earlier BuyDates screws up the TradeDayIndex values.
				// TODO(jsd): We could probably fix this better by simply reindexing the TradeDayIndex values and filling in the holes.
				api.DeleteHistory(s.Symbol)
			}

			// Fetch latest data for new symbol:
			fetchLatest(api, s.Symbol)

			rsp = "ok"

		case "/stock/remove":
			tmp := struct {
				ID int64 `json:"id"`
			}{}
			parsePostJson(r, &tmp)

			stockID := stocks.StockID(tmp.ID)

			err = api.RemoveStock(stockID)
			panicIf(err)

			rsp = "ok"

		default:
			rspcode = 404
			rsperr = fmt.Errorf("Invalid API url")
		}
	}
}
