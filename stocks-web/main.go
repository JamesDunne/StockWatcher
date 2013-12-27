package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	//"net/url"
	"os"
	"os/signal"
	"path"
	"syscall"
	"time"
)

// sqlite related imports:
import (
	"database/sql"
	//"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Our own packages:
import (
	//"github.com/JamesDunne/StockWatcher/dbutil"
	//"github.com/JamesDunne/StockWatcher/mailutil"
	"github.com/JamesDunne/StockWatcher/stocksAPI"
)

// Where to serve static files from:
var fsRoot = "./root/"
var dbPath string

// ----------------------- Secured section:

// DB record from StockOwned join User:
type dbStock struct {
	UserID                  int `db:"UserID"`
	UserNotificationTimeout int `db:"UserNotificationTimeout"` // timeout in seconds

	StockOwnedID             int            `db:"StockOwnedID"`
	Symbol                   string         `db:"Symbol"`
	PurchasePrice            string         `db:"PurchasePrice"`
	PurchaseDate             string         `db:"PurchaseDate"`
	StopPercent              string         `db:"StopPercent"`
	StopLastNotificationDate sql.NullString `db:"StopLastNotificationDate"`
}

type dbUser struct {
	UserID              int    `db:"UserID"`
	Email               string `db:"Email"`
	Name                string `db:"Name"`
	NotificationTimeout int    `db:"NotificationTimeout"` // timeout in seconds
}

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

	switch r.URL.Path {
	case "/register":
		http.ServeFile(w, r, path.Join(fsRoot, "register.html"))

	case "/dash":
		// Dashboard UI:
		http.ServeFile(w, r, path.Join(fsRoot, "dash.html"))

	default:
		http.NotFound(w, r)
	}
}

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
	api, err := stocksAPI.NewAPI(dbPath)
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
	switch r.URL.Path {
	case "/user/who":
		rsp = apiuser

	case "/user/register":
		// Register new user with primary email.

		if apiuser == nil {
			// Add user:
			apiuser = &stocksAPI.User{
				PrimaryEmail:        webuser.Email,
				Name:                webuser.FullName,
				NotificationTimeout: time.Minute,
			}
			err = api.AddUser(apiuser)
			if err != nil {
				rsperr = err
				return
			}
		}
		rsp = apiuser

	case "/user/join":
		// Join to existing user with secondary email.
		rsperr = fmt.Errorf("TODO")

	case "/owned/list":
		// Get list of owned stocks w/ details.
		owned, err := api.GetOwnedStocksByUser(apiuser.UserID)
		if err != nil {
			log.Println(err)
			http.Error(w, "Fail GetOwnedStocksByUser", http.StatusInternalServerError)
			return
		}
		rsp = owned

	case "/owned/add":
		// Add owned stock.
		rsperr = fmt.Errorf("TODO")

	case "/owned/disable":
		// Disable notifications.
		rsperr = fmt.Errorf("TODO")

	case "/owned/enable":
		// Enable notifications.
		rsperr = fmt.Errorf("TODO")

	case "/watched/list":
		// Get list of watched stocks w/ details.
		watched, err := api.GetWatchedStocksByUser(apiuser.UserID)
		if err != nil {
			log.Println(err)
			http.Error(w, "Fail GetWatchedStocksByUser", http.StatusInternalServerError)
			return
		}
		rsp = watched

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

// ----------------------- Unsecured section:

// Handles all other requests including root "/":
func rootHandler(w http.ResponseWriter, r *http.Request) {
	// Root page redirects to /auth/login if unauthenticated:
	if r.URL.Path == "/" {
		if isAuthenticated(r) {
			// UI dashboard:
			http.Redirect(w, r, "/ui/dash", http.StatusFound)
		} else {
			// Login page:
			http.Redirect(w, r, "/auth/login", http.StatusFound)
		}
		return
	}

	// TODO: is this wise? Probably not.
	//http.ServeFile(w, r, r.URL.Path)

	// 404 as a catch-all:
	http.NotFound(w, r)
}

// Entry point:
func main() {
	// Define our commandline flags:
	socketType := flag.String("t", "tcp", "socket type to listen on: 'unix', 'tcp', 'udp'")
	socketAddr := flag.String("l", ":8080", "address to listen on")
	fs := flag.String("fs", "./root", "Root directory of static files to serve")
	dbPathArg := flag.String("db", "./stocks.db", "Path to stocks.db database")
	webHostArg := flag.String("host", "localhost:8080", "Host name of server; used for HTTP redirects")
	mailServerArg := flag.String("mail-server", "localhost:25", "Address of SMTP server to use for sending email")

	// Parse the flags and set values:
	flag.Parse()
	fsRoot = *fs
	dbPath = *dbPathArg
	WebHost = *webHostArg
	_ = *mailServerArg

	// Create the socket to listen on:
	l, err := net.Listen(*socketType, *socketAddr)
	if err != nil {
		log.Fatal(err)
		return
	}

	// NOTE(jsd): Unix sockets must be removed before being reused.

	// Handle common process-killing signals so we can gracefully shut down:
	// TODO(jsd): Go does not catch Windows' process kill signals (yet?)
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGQUIT)
	go func(c chan os.Signal) {
		// Wait for a signal:
		sig := <-c
		log.Printf("Caught signal '%s': shutting down.\n", sig)

		// Stop listening:
		l.Close()

		// Delete the unix socket, if applicable:
		if *socketType == "unix" {
			os.Remove(*socketAddr)
		}

		// And we're done:
		os.Exit(0)
	}(sigc)

	// Declare HTTP handlers:

	// Authentication section:
	http.Handle("/auth/", http.StripPrefix("/auth", http.HandlerFunc(authHandler)))

	// Secured section:
	http.Handle("/ui/", RequireAuth(http.StripPrefix("/ui", http.HandlerFunc(uiHandler))))
	http.Handle("/api/", RequireAuth(http.StripPrefix("/api", http.HandlerFunc(apiHandler))))

	// Unsecured section:
	// For serving static files:
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir(fsRoot))))
	// Catch-all handler:
	http.Handle("/", http.HandlerFunc(rootHandler))

	// Start the HTTP server and block the main goroutine:
	log.Fatal(http.Serve(l, nil))
}
