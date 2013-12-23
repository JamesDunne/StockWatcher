package main

import (
	//"bufio"
	//"fmt"
	//"io"
	//"path/filepath"
	//	"ioutil"
	"flag"
	"log"
	//"mime"
	"net"
	"net/http"
	//"net/url"
	"os"
	"os/signal"
	//"path"
	//"strconv"
	"syscall"
)

import "github.com/JamesDunne/go-openid"

// openid authentication store: (total crap; leaks memory - replace)
var nonceStore = &openid.SimpleNonceStore{Store: make(map[string][]*openid.Nonce)}
var discoveryCache = &openid.SimpleDiscoveryCache{}

// Handles /auth/* requests:
func authHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "login":
		if r.Method == "GET" {
			http.ServeFile(w, r, "index.html")
			return
		} else if r.Method == "POST" {
			// Redirect to openid provider and instruct them to come back here at /auth/openid:
			if url, err := openid.RedirectUrl(
				r.FormValue("id"),
				"http://"+r.URL.Host+"/auth/openid",
				""); err == nil {
				http.Redirect(w, r, url, 303)
			} else {
				log.Print(err)
			}
			return
		}
	case "openid":
		// Redirected from openid provider to here:
		fullUrl := "http://" + r.Host + r.URL.String()
		log.Print(fullUrl)
		id, err := openid.Verify(fullUrl, discoveryCache, nonceStore)
		if err != nil {
			log.Println(err)
			http.Error(w, "Not Authorized", 401)
		}
		log.Println(id)
		return
	}
}

// Handles /api/* requests:
func apiHandler(w http.ResponseWriter, r *http.Request) {

}

func main() {
	// Expect commandline arguments to specify:
	//   <listen socket type> : "unix" or "tcp" type of socket to listen on
	//   <listen address>     : network address to listen on if "tcp" or path to socket if "unix"
	socketType := flag.String("t", "tcp", "socket type to listen on: 'unix', 'tcp', 'udp'")
	socketAddr := flag.String("l", ":8080", "address to listen on")
	flag.Parse()

	// Create the socket to listen on:
	l, err := net.Listen(*socketType, *socketAddr)
	if err != nil {
		log.Fatal(err)
		return
	}

	// NOTE(jsd): Unix sockets must be unlink()ed before being reused again.

	// Handle common process-killing signals so we can gracefully shut down:
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, os.Kill, syscall.SIGTERM, syscall.SIGQUIT)
	go func(c chan os.Signal) {
		// Wait for a signal:
		sig := <-c
		log.Printf("Caught signal '%s': shutting down.", sig)
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

	// All "/api/" requests are special JSON handlers
	http.Handle("/auth/", http.StripPrefix("/auth/", http.HandlerFunc(authHandler)))
	http.Handle("/api/", http.StripPrefix("/api/", http.HandlerFunc(apiHandler)))
	http.Handle("/", http.FileServer(http.Dir("./root/")))

	// Start the HTTP server:
	log.Fatal(http.Serve(l, nil))
}
