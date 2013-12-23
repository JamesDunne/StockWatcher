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
	"net/url"
	"os"
	"os/signal"
	//"path"
	//"strconv"
	"syscall"
)

import openid "github.com/JamesDunne/go-openid"

// openid authentication store: (total crap; leaks memory - replace)
var nonceStore = &openid.SimpleNonceStore{Store: make(map[string][]*openid.Nonce)}
var discoveryCache = &openid.SimpleDiscoveryCache{}

// Handles /auth/* requests:
func authHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/login":
		if r.Method == "GET" {
			http.ServeFile(w, r, "index.html")
			return
		} else if r.Method == "POST" {
			// Redirect to openid provider and instruct them to come back here at /auth/openid:
			if url, err := openid.RedirectUrl(
				r.FormValue("id"),
				"http://"+r.Host+"/auth/openid",
				"",
				map[string]string{
					"openid.ns.ax":             "http://openid.net/srv/ax/1.0",
					"openid.ax.mode":           "fetch_request",
					"openid.ax.required":       "firstname,lastname,username,language,email",
					"openid.ax.type.username":  "http://axschema.org/namePerson/friendly",
					"openid.ax.type.language":  "http://axschema.org/pref/language",
					"openid.ax.type.lastname":  "http://axschema.org/namePerson/last",
					"openid.ax.type.firstname": "http://axschema.org/namePerson/first",
					"openid.ax.type.email":     "http://axschema.org/contact/email",
				}); err == nil {
				http.Redirect(w, r, url, 303)
			} else {
				log.Print(err)
			}
			return
		}
	case "/openid":
		// Redirected from openid provider to here:
		verifyUrl := &url.URL{Scheme: "http", Host: r.Host, Path: "auth" + r.URL.Path, RawQuery: r.URL.RawQuery}
		verify := verifyUrl.String()
		log.Println(verify)

		id, err := openid.Verify(verify, discoveryCache, nonceStore)
		if err != nil {
			log.Println(err)
			http.Error(w, "Not Authorized", http.StatusUnauthorized)
			return
		}
		log.Println(id)

		// Extract useful user information from query string:
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			http.Error(w, "Could not parse query", http.StatusInternalServerError)
			return
		}

		log.Println(q)
		log.Printf("%s %s <%s>\n", q.Get("openid.ext1.value.firstname"), q.Get("openid.ext1.value.lastname"), q.Get("openid.ext1.value.email"))
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
	http.Handle("/auth/", http.StripPrefix("/auth", http.HandlerFunc(authHandler)))
	http.Handle("/api/", http.StripPrefix("/api", http.HandlerFunc(apiHandler)))
	http.Handle("/", http.FileServer(http.Dir("./root/")))

	// Start the HTTP server:
	log.Fatal(http.Serve(l, nil))
}
