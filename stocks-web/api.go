// api.go
package main

import (
	"encoding/json"
	"fmt"
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
			rsperr = fmt.Errorf("TODO")

		case "/watched/list":
			// Get list of watched stocks w/ details.
			rsperr = fmt.Errorf("TODO")

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

		case "/owned/add":
			// Add owned stock.
			rsperr = fmt.Errorf("TODO")

		case "/owned/disable":
			// Disable notifications.
			rsperr = fmt.Errorf("TODO")

		case "/owned/enable":
			// Enable notifications.
			rsperr = fmt.Errorf("TODO")

		case "/watched/add":
			// Add watched stock.
			rsperr = fmt.Errorf("TODO")

		case "/watched/disable":
			// Disable notifications.
			rsperr = fmt.Errorf("TODO")

		case "/watched/enable":
			// Enable notifications.
			rsperr = fmt.Errorf("TODO")

		default:
			rspcode = 404
			rsperr = fmt.Errorf("Invalid API url")
		}
	}
}
