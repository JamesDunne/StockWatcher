// ui.go
package main

import (
	//"fmt"
	"html/template"
	"log"
	"net/http"
	//"net/url"
	//"path"
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
	// Get authenticated user data:
	webuser := getUserData(r)

	// Determine if user is registered:
	api, err := stocksAPI.NewAPI(dbPath)
	if err != nil {
		log.Println(err)
		http.Error(w, "Could not open stocks database!", http.StatusInternalServerError)
		return
	}
	defer api.Close()

	// Handle panic()s:
	defer func() {
		if err := recover(); err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
	}()

	// Find user:
	apiuser, err := api.GetUserByEmail(webuser.Email)
	if apiuser == nil || err != nil {
		if r.URL.Path != "/register" {
			http.Redirect(w, r, "/ui/register", http.StatusFound)
			return
		}
	}

	// Define our model:

	switch r.URL.Path {
	case "/register":
		// Fetch data to be used by the template:
		model := struct {
			User *stocksAPI.User
		}{
			User: apiuser,
		}
		uiTmpl.ExecuteTemplate(w, "register", model)
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
