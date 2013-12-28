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

func getWatched(api *stocksAPI.API, userID stocksAPI.UserID) []stocksAPI.WatchedDetails {
	watched, err := api.GetWatchedDetailsForUser(userID)
	if err != nil {
		panic(err)
	}
	return watched
}

// ----------------------- Secured section:

const dateFmt = "2006-01-02"

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
			User    *stocksAPI.User
			Owned   []stocksAPI.OwnedDetails
			Watched []stocksAPI.WatchedDetails
		}{
			User:    apiuser,
			Owned:   getOwned(api, apiuser.UserID),
			Watched: getWatched(api, apiuser.UserID),
		}

		err := uiTmpl.ExecuteTemplate(w, "dash", model)
		if err != nil {
			panic(err)
		}
		return

	case "/owned/disable":
		if r.Method == "POST" {
			// Parse form data:
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}

			ownedIDint, err := strconv.ParseInt(r.PostForm.Get("id"), 10, 0)
			if err != nil {
				panic(err)
			}
			ownedID := stocksAPI.OwnedID(ownedIDint)

			err = api.DisableOwned(ownedID)
			if err != nil {
				panic(err)
			}

			w.WriteHeader(200)
		}
		return

	case "/owned/enable":
		if r.Method == "POST" {
			// Parse form data:
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}

			ownedIDint, err := strconv.ParseInt(r.PostForm.Get("id"), 10, 0)
			if err != nil {
				panic(err)
			}
			ownedID := stocksAPI.OwnedID(ownedIDint)

			err = api.EnableOwned(ownedID)
			if err != nil {
				panic(err)
			}

			w.WriteHeader(200)
		}
		return

	case "/owned/remove":
		if r.Method == "POST" {
			// Parse form data:
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}

			ownedIDint, err := strconv.ParseInt(r.PostForm.Get("id"), 10, 0)
			if err != nil {
				panic(err)
			}
			ownedID := stocksAPI.OwnedID(ownedIDint)

			err = api.RemoveOwned(ownedID)
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
				User *stocksAPI.User
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

			symbol := r.PostForm.Get("symbol")
			if symbol == "" {
				http.Error(w, fmt.Sprintf("Symbol required"), http.StatusBadRequest)
				return
			}
			buyDate := r.PostForm.Get("buyDate")
			if _, err := time.Parse(dateFmt, buyDate); err != nil {
				log.Println(err)
				http.Error(w, fmt.Sprintf("Invalid buy date; must be in '%s' format", dateFmt), http.StatusBadRequest)
				return
			}
			buyPrice := stocksAPI.ToRat(r.PostForm.Get("buyPrice"))
			if buyPrice == nil {
				http.Error(w, fmt.Sprintf("Buy price required"), http.StatusBadRequest)
				return
			}
			shares, err := strconv.ParseInt(r.PostForm.Get("shares"), 10, 0)
			if err != nil {
				log.Println(err)
				http.Error(w, fmt.Sprintf("Invalid shares value"), http.StatusBadRequest)
				return
			}
			stopPercent := stocksAPI.ToRat(r.PostForm.Get("stopPercent"))
			if stopPercent == nil {
				http.Error(w, fmt.Sprintf("T-Stop Percent required"), http.StatusBadRequest)
				return
			}

			// Add the owned stock:
			err = api.AddOwned(apiuser.UserID, symbol, buyDate, buyPrice, int(shares), stopPercent)
			if err != nil {
				panic(err)
			}

			http.Redirect(w, r, "/ui/dash", http.StatusFound)
		}
		return

		// ----------------------

	case "/watched/disable":
		if r.Method == "POST" {
			// Parse form data:
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}

			watchedIDint, err := strconv.ParseInt(r.PostForm.Get("id"), 10, 0)
			if err != nil {
				panic(err)
			}
			watchedID := stocksAPI.WatchedID(watchedIDint)

			err = api.DisableWatched(watchedID)
			if err != nil {
				panic(err)
			}

			w.WriteHeader(200)
		}
		return

	case "/watched/enable":
		if r.Method == "POST" {
			// Parse form data:
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}

			watchedIDint, err := strconv.ParseInt(r.PostForm.Get("id"), 10, 0)
			if err != nil {
				panic(err)
			}
			watchedID := stocksAPI.WatchedID(watchedIDint)

			err = api.EnableWatched(watchedID)
			if err != nil {
				panic(err)
			}

			w.WriteHeader(200)
		}
		return

	case "/watched/remove":
		if r.Method == "POST" {
			// Parse form data:
			err := r.ParseForm()
			if err != nil {
				panic(err)
			}

			watchedIDint, err := strconv.ParseInt(r.PostForm.Get("id"), 10, 0)
			if err != nil {
				panic(err)
			}
			watchedID := stocksAPI.WatchedID(watchedIDint)

			err = api.RemoveWatched(watchedID)
			if err != nil {
				panic(err)
			}

			w.WriteHeader(200)
		}
		return

	case "/watched/add":
		if r.Method == "GET" {
			// Data to be used by the template:
			model := struct {
				User *stocksAPI.User
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

			symbol := r.PostForm.Get("symbol")
			if symbol == "" {
				http.Error(w, fmt.Sprintf("Symbol required"), http.StatusBadRequest)
				return
			}
			buyDate := r.PostForm.Get("buyDate")
			if _, err := time.Parse(dateFmt, buyDate); err != nil {
				log.Println(err)
				http.Error(w, fmt.Sprintf("Invalid buy date; must be in '%s' format", dateFmt), http.StatusBadRequest)
				return
			}
			buyPrice := stocksAPI.ToRat(r.PostForm.Get("buyPrice"))
			if buyPrice == nil {
				http.Error(w, fmt.Sprintf("Buy price required"), http.StatusBadRequest)
				return
			}
			//shares, err := strconv.ParseInt(r.PostForm.Get("shares"), 10, 0)
			//if err != nil {
			//	log.Println(err)
			//	http.Error(w, fmt.Sprintf("Invalid shares value"), http.StatusBadRequest)
			//	return
			//}
			stopPercent := stocksAPI.ToRat(r.PostForm.Get("stopPercent"))
			if stopPercent == nil {
				http.Error(w, fmt.Sprintf("T-Stop Percent required"), http.StatusBadRequest)
				return
			}

			// Add the owned stock:
			err = api.AddWatched(apiuser.UserID, symbol, buyDate, buyPrice, stopPercent)
			if err != nil {
				panic(err)
			}

			http.Redirect(w, r, "/ui/dash", http.StatusFound)
		}
		return
	}

	http.NotFound(w, r)
	return
}
