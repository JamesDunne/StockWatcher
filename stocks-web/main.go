package main

import (
	"flag"
	//"fmt"
	"html/template"
	"log"
	"net"
	"net/http"
	//"net/url"
	"os"
	"os/signal"
	"path"
	"syscall"
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
	"github.com/JamesDunne/StockWatcher/mailutil"
	//"github.com/JamesDunne/StockWatcher/stocks"
	"github.com/JamesDunne/go-fsnotify"
)

// Where to serve static files from:
var fsRoot = "./root/"
var dbPath string

// Override this with the production host name, e.g. stocks.bittwiddlers.org (port optional):
var webHost = "localhost:8080"

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
	fs := flag.String("fs", "./", "Root directory of served files and templates")
	dbPathArg := flag.String("db", "./stocks.db", "Path to stocks.db database")
	webHostArg := flag.String("host", "localhost:8080", "Host name of server; used for HTTP redirects")
	mailServerArg := flag.String("mail-server", "localhost:25", "Address of SMTP server to use for sending email")

	// Parse the flags and set values:
	flag.Parse()
	fsRoot = *fs
	dbPath = *dbPathArg
	webHost = *webHostArg
	mailutil.Server = *mailServerArg

	// Parse template files:
	tmplPath := path.Join(fsRoot, "templates")
	ui, err := template.New("ui").ParseGlob(path.Join(tmplPath, "*.tmpl"))
	if err != nil {
		log.Fatal(err)
		return
	}
	uiTmpl = ui

	// Watch template directory for file changes:
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	defer func() { watcher.RemoveWatch(tmplPath); watcher.Close() }()

	// Process watcher events
	go func() {
		for {
			select {
			case ev := <-watcher.Event:
				if ev == nil {
					break
				}
				//log.Println("event:", ev)

				// Update templates:
				var err error
				ui, err := template.New("ui").ParseGlob(path.Join(tmplPath, "*.tmpl"))
				if err != nil {
					log.Println(err)
					break
				}
				uiTmpl = ui
			case err := <-watcher.Error:
				if err == nil {
					break
				}
				log.Println("watcher error:", err)
			}
		}
	}()

	// Watch template file for changes:
	watcher.Watch(tmplPath)

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
	http.Handle("/static/", http.StripPrefix("/static", http.FileServer(http.Dir(path.Join(fsRoot, "static")))))
	// Catch-all handler:
	http.Handle("/", http.HandlerFunc(rootHandler))

	// Start the HTTP server and block the main goroutine:
	log.Fatal(http.Serve(l, nil))
}
