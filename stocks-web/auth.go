package main

import (
	"encoding/json"
	"fmt"
	openid "github.com/JamesDunne/go-openid"
	"log"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

// openid authentication store: (total crap; leaks memory - replace)
var nonceStore = &openid.SimpleNonceStore{Store: make(map[string][]*openid.Nonce)}
var discoveryCache = &openid.SimpleDiscoveryCache{}

// Stored user data in a cookie, encoded as JSON:
type UserCookieData struct {
	Email    string `json:"email"`
	FullName string `json:"full"`
	TimeZone string `json:"tz"`
}

// Sets the authentication cookie:
func setAuthCookie(w http.ResponseWriter, userData *UserCookieData) {
	if userData == nil {
		panic("setAuthCookie: userData cannot be nil!")
	}

	userJsonBytes, err := json.Marshal(userData)
	if err != nil {
		log.Println(err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// Set authentication cookie:
	authCookie := &http.Cookie{
		Name:    "auth",
		Path:    "/",
		Expires: time.Now().Add(time.Hour * time.Duration(24*14)),
		Value:   url.QueryEscape(string(userJsonBytes)),
	}
	http.SetCookie(w, authCookie)
}

// Clears the authentication cookie (i.e. log out):
func clearAuthCooke(w http.ResponseWriter) {
	// Removing a cookie is tantamount to setting the expiration date in the past.
	authCookie := &http.Cookie{
		Name:    "auth",
		Path:    "/",
		Expires: time.Now().Add(time.Hour * time.Duration(24*-2)),
		Value:   "",
	}
	http.SetCookie(w, authCookie)
}

// Handles /auth/* requests:
func authHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/login":
		if r.Method == "GET" {
			// Present login page:
			http.ServeFile(w, r, path.Join(fsRoot, "login.html"))
			return
		} else if r.Method == "POST" {
			// Redirect to openid provider and instruct them to come back here at /auth/openid:
			if url, err := openid.RedirectUrl(
				r.FormValue("id"),
				"http://"+webHost+"/auth/openid",
				"",
				map[string]string{
					"openid.ns.ax":             "http://openid.net/srv/ax/1.0",
					"openid.ax.mode":           "fetch_request",
					"openid.ax.required":       "email,timezone,fullname,friendly,firstname,lastname",
					"openid.ax.type.email":     "http://axschema.org/contact/email",
					"openid.ax.type.timezone":  "http://axschema.org/pref/timezone",
					"openid.ax.type.fullname":  "http://axschema.org/namePerson",
					"openid.ax.type.friendly":  "http://axschema.org/namePerson/friendly",
					"openid.ax.type.lastname":  "http://axschema.org/namePerson/last",
					"openid.ax.type.firstname": "http://axschema.org/namePerson/first",
				}); err == nil {
				http.Redirect(w, r, url, http.StatusSeeOther)
			} else {
				log.Print(err)
			}
			return
		}

	case "/logout":
		// Logout simply removes the auth cookie and redirects to '/auth/login':
		clearAuthCooke(w)
		http.Redirect(w, r, "/auth/login", http.StatusFound)
		return

	case "/openid":
		// Redirected from openid provider to here:
		verifyUrl := (&url.URL{Scheme: "http", Host: webHost, Path: path.Join("auth", r.URL.Path), RawQuery: r.URL.RawQuery}).String()
		//log.Println(verifyUrl)

		// Don't care about the `id` coming back; it's useless.
		if _, err := openid.Verify(verifyUrl, discoveryCache, nonceStore); err != nil {
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

		// Search for the ax namespace alias; providers use different aliases:
		var axAlias string
		for key, valArray := range q {
			if len(valArray) == 0 {
				continue
			}
			val := valArray[0]

			if strings.HasPrefix(key, "openid.ns.") && val == "http://openid.net/srv/ax/1.0" {
				axAlias = strings.TrimPrefix(key, "openid.ns.")
				break
			}
		}
		if axAlias == "" {
			log.Printf("Could not determine OpenID ax namespace alias from query string response!\n%#v\n", q)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		// Extract the ax attributes into a map:
		nsTypePrefix := "openid." + axAlias + ".type."
		nsValuePrefix := "openid." + axAlias + ".value."

		// Create a map of aliases to namespaces based on 'openid.alias.type' values:
		nses := make(map[string]string)
		for key, valArray := range q {
			if len(valArray) == 0 {
				continue
			}

			if strings.HasPrefix(key, nsTypePrefix) {
				alias := strings.TrimPrefix(key, nsTypePrefix)
				ns := valArray[0]

				nses[alias] = ns
			}
		}

		// Create a map of namespaces to values based on 'openid.alias.value' values:
		//log.Printf("ax:\n")
		ax := make(map[string]string)
		for key, valArray := range q {
			if len(valArray) == 0 {
				continue
			}

			if strings.HasPrefix(key, nsValuePrefix) {
				alias := strings.TrimPrefix(key, nsValuePrefix)
				ns := nses[alias]
				val := valArray[0]

				ax[ns] = val

				//log.Printf("  %s: '%s'\n", strings.TrimPrefix(ns, "http://axschema.org/"), val)
			}
		}

		// This is the map of namespaces to aliases:
		//  "http://axschema.org/contact/email":        "email"
		//  "http://axschema.org/pref/timezone":        "timezone"
		//  "http://axschema.org/namePerson":           "fullname"
		//  "http://axschema.org/namePerson/friendly":  "friendly"
		//  "http://axschema.org/namePerson/last":      "lastname"
		//  "http://axschema.org/namePerson/first":     "firstname"

		// Create user data struct for auth cookie:
		userData := &UserCookieData{
			Email:    ax["http://axschema.org/contact/email"],
			TimeZone: ax["http://axschema.org/pref/timezone"],
		}

		// Determine full name using some stupid rules because Google only provides first/last name and Yahoo only provides full name.
		// Other OpenID providers may do even more stupid things; I haven't tested anything besides Google and Yahoo.
		if firstname, ok := ax["http://axschema.org/namePerson/first"]; ok {
			if lastname, ok := ax["http://axschema.org/namePerson/last"]; ok {
				userData.FullName = fmt.Sprintf("%s %s", firstname, lastname)
			} else {
				// Just use firstname:
				userData.FullName = firstname
			}
		} else if fullname, ok := ax["http://axschema.org/namePerson"]; ok {
			userData.FullName = fullname
		} else if friendly, ok := ax["http://axschema.org/namePerson/friendly"]; ok {
			// Yahoo provides this as first name; Google does not provide at all.
			userData.FullName = friendly
		}

		if userData.FullName == "" {
			log.Printf("Could not determine full name from OpenID ax attributes!\n%#v\n", ax)
			// Not crucial, so don't error out.
		}
		if userData.TimeZone == "" {
			// Google does not provide time zones. Yahoo does but they guessed wrong when I created my account. Blunderful.
			userData.TimeZone = "America/Chicago"
		}

		// Set authentication cookie:
		setAuthCookie(w, userData)

		// Redirect to dashboard:
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
	// Require that the 'auth' cookie be set:
	_, err := r.Cookie("auth")
	if err != nil {
		http.Error(w, "Not Authenticated", http.StatusUnauthorized)
		return
	}

	// TODO: need to verify cookie not expired?

	// Handlers further down the chain should use `getUserData` to decode the auth cookie
	// and get user info.

	// Pass to the next handler in the chain:
	h.handler.ServeHTTP(w, r)
}

func isAuthenticated(r *http.Request) bool {
	_, err := r.Cookie("auth")
	if err != nil {
		return false
	}

	return true
}

// Utility function to get authenticated user data:
func getUserData(r *http.Request) (userData *UserCookieData) {
	authCookie, err := r.Cookie("auth")
	if err != nil {
		return nil
	}

	userJsonStr, err := url.QueryUnescape(authCookie.Value)
	if err != nil {
		log.Println(err)
		return nil
	}

	userData = new(UserCookieData)
	if err = json.Unmarshal([]byte(userJsonStr), userData); err != nil {
		log.Println(err)
		return nil
	}

	return
}
