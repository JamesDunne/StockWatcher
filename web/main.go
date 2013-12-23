package main

import (
	"encoding/json"
	"flag"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

import openid "github.com/JamesDunne/go-openid"

// openid authentication store: (total crap; leaks memory - replace)
var nonceStore = &openid.SimpleNonceStore{Store: make(map[string][]*openid.Nonce)}
var discoveryCache = &openid.SimpleDiscoveryCache{}

type UserCookieData struct {
	Email string `json:"email"`
	First string `json:"first"`
	Last  string `json:"last"`
}

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

		// Don't care about the `id` coming back; it's useless.
		if _, err := openid.Verify(verify, discoveryCache, nonceStore); err != nil {
			log.Println(err)
			http.Error(w, "Not Authorized", http.StatusUnauthorized)
			return
		}

		// Extract the much more useful user information from the query string:
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		log.Println(q)

		// Create user data JSON for cookie:
		userData := &UserCookieData{
			Email: q.Get("openid.ext1.value.email"),
			First: q.Get("openid.ext1.value.firstname"),
			Last:  q.Get("openid.ext1.value.lastname"),
		}
		userJsonBytes, err := json.Marshal(userData)
		if err != nil {
			log.Println(err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
		log.Println(string(userJsonBytes))

		// Set authentication cookie:
		authCookie := &http.Cookie{
			Name:    "auth",
			Path:    "/",
			Expires: time.Now().Add(time.Hour * time.Duration(24*14)),
			Value:   url.QueryEscape(string(userJsonBytes)),
		}
		http.SetCookie(w, authCookie)
		http.Redirect(w, r, "/ui/dash", http.StatusFound)
		return
	}
}

// Handler to require authentication cookie:
type requireAuthHandler struct {
	handler http.Handler
}

func RequireAuth(h http.Handler) requireAuthHandler {
	return requireAuthHandler{handler: h}
}

func (h requireAuthHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	authCookie, err := r.Cookie("auth")
	if err != nil {
		log.Println(err)
		http.Error(w, "Not Authenticated", http.StatusUnauthorized)
		return
	}

	userJsonStr, err := url.QueryUnescape(authCookie.Value)
	if err != nil {
		log.Println(err)
		http.Error(w, "Not Authenticated", http.StatusUnauthorized)
		return
	}

	// TODO(jsd): Need a place to store user details for the request context!
	log.Println(userJsonStr)
	_ = userJsonStr

	// Pass to the next handler in the chain:
	h.handler.ServeHTTP(w, r)
}

// ----------------------- Secured section:

// Handles /ui/* requests to present HTML UI to the user:
func uiHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/dash":
		// Dashboard UI
	}
}

// Handles /api/* requests for JSON API:
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
	http.Handle("/auth/", http.StripPrefix("/auth", http.HandlerFunc(authHandler)))

	// Secured section:
	http.Handle("/ui/", RequireAuth(http.StripPrefix("/ui", http.HandlerFunc(uiHandler))))
	http.Handle("/api/", RequireAuth(http.StripPrefix("/api", http.HandlerFunc(apiHandler))))

	// Catch-all handler to serve static files:
	http.Handle("/", http.FileServer(http.Dir("./root/")))

	// Start the HTTP server and block until killed:
	log.Fatal(http.Serve(l, nil))
}
