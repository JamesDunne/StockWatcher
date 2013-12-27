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

	// Find user:
	apiuser, err := api.GetUserByEmail(webuser.Email)
	if apiuser == nil || err != nil {
		if r.URL.Path != "/register" {
			http.Redirect(w, r, "/ui/register", http.StatusFound)
			return
		}
	}

	// Define our model:
	model := struct {
		User *stocksAPI.User
	}{
		User: apiuser,
	}

	switch r.URL.Path {
	case "/register":
		uiTmpl.ExecuteTemplate(w, "register", model)
		return

	case "/dash":
		uiTmpl.ExecuteTemplate(w, "dash", model)
		return

	default:
		http.NotFound(w, r)
	}
}
