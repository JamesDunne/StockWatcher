package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	//"net/url"
	"os"
	"os/signal"
	"syscall"
	//"time"
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
	"github.com/JamesDunne/StockWatcher/stocksdb"
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

// Handles /ui/* requests to present HTML UI to the user:
func uiHandler(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user data:
	user := getUserData(r)

	switch r.URL.Path {
	case "/dash":
		// Dashboard UI:
		db, err := stocksdb.Open(dbPath)
		if err != nil {
			log.Println(err)
			http.Error(w, "Could not open stocks database!", http.StatusInternalServerError)
			return
		}
		defer db.Close()

		// Query what stocks are purchased by this user:
		stocks := make([]dbStock, 0, 4) // make(type, len, capacity)
		if err = db.Select(&stocks, `
select s.UserID, u.NotificationTimeout AS UserNotificationTimeout
     , s.rowid as StockOwnedID, s.Symbol, s.PurchaseDate, s.PurchasePrice, s.StopPercent, s.StopLastNotificationDate
from StockOwned as s
join User as u on u.rowid = s.UserID
where s.IsStopEnabled = 1
  and u.Email = ?1`, user.Email); err != nil {
			log.Println(err)
			http.Error(w, "Query failed!", http.StatusInternalServerError)
			return
		}

		// Render an HTML response:
		w.Header().Set("Content-Type", `text/html; charset="utf-8"`)
		w.WriteHeader(200)

		w.Write([]byte(fmt.Sprintf(`<!DOCTYPE html>

<html>
<body>
<h3>Welcome, %s %s &lt;%s&gt;!</h3>
<p>These are your purchased stocks:<br>
%+v
</p>
Click <a href="/auth/logout">here</a> to log out.
</body>
</html>`,
			user.First,
			user.Last,
			user.Email,
			stocks,
		)))
		return
	}
}

// Handles /api/* requests for JSON API:
func apiHandler(w http.ResponseWriter, r *http.Request) {
	// TODO
	http.NotFound(w, r)
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
	root := flag.String("fs", "./root", "Root directory of static files to serve")
	dbPathArg := flag.String("db", "./stocks.db", "Path to stocks.db database")
	mailServerArg := flag.String("mail-server", "localhost:25", "Address of SMTP server to use for sending email")

	// Parse the flags and set values:
	flag.Parse()
	fsRoot = *root
	dbPath = *dbPathArg
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
