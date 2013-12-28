// ui.go
package main

import (
	//"fmt"
	"html/template"
	"log"
	"net/http"
	//"net/url"
	//"path"
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
	"github.com/JamesDunne/StockWatcher/stocksAPI"
)

// utilities:

func getOwned(api *stocksAPI.API, userID stocksAPI.UserID) []stocksAPI.OwnedDetails {
	owned, err := api.GetOwnedDetailsForUser(userID)
	if err != nil {
		panic(err)
	}
	return owned
}

// ----------------------- Secured section:

var uiTmpl *template.Template

// Handles /ui/* requests to present HTML UI to the user:
func uiHandler(w http.ResponseWriter, r *http.Request) {
	// Get API ready:
	api, err := stocksAPI.NewAPI(dbPath)
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
			http.Error(w, "Internal server error", http.StatusInternalServerError)
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
				User    *stocksAPI.User
			}{
				WebUser: webuser,
				User:    apiuser,
			}

			uiTmpl.ExecuteTemplate(w, "register", model)
		} else {
			// Assume POST.

			// Add user:
			apiuser = &stocksAPI.User{
				PrimaryEmail:        webuser.Email,
				Name:                webuser.FullName,
				NotificationTimeout: time.Hour * time.Duration(24),
			}

			err = api.AddUser(apiuser)
			if err != nil {
				panic(err)
			}

			http.Redirect(w, r, "/ui/dash", http.StatusFound)
		}
		return

	case "/dash":
		// Fetch data to be used by the template:
		model := struct {
			User  *stocksAPI.User
			Owned []stocksAPI.OwnedDetails
		}{
			User:  apiuser,
			Owned: getOwned(api, apiuser.UserID),
		}

		err := uiTmpl.ExecuteTemplate(w, "dash", model)
		if err != nil {
			panic(err)
		}
		return

	default:
		http.NotFound(w, r)
	}
}
