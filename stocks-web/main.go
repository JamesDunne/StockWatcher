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

// Where to serve static files from:
var fsRoot = "./root/"

// ----------------------- Secured section:

// Handles /ui/* requests to present HTML UI to the user:
func uiHandler(w http.ResponseWriter, r *http.Request) {
	// Get authenticated user data:
	user := getUserData(r)

	switch r.URL.Path {
	case "/dash":
		// Dashboard UI
		_ = user
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprintf("Welcome, %s %s <%s>!", user.First, user.Last, user.Email)))
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

	// Parse the flags and set values:
	flag.Parse()

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
