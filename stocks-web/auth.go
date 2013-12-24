package main

import (
	"encoding/json"
	openid "github.com/JamesDunne/go-openid"
	"log"
	"net/http"
	"net/url"
	"time"
)

// openid authentication store: (total crap; leaks memory - replace)
var nonceStore = &openid.SimpleNonceStore{Store: make(map[string][]*openid.Nonce)}
var discoveryCache = &openid.SimpleDiscoveryCache{}

// Stored user data in a cookie, encoded as JSON:
type UserCookieData struct {
	Email string `json:"email"`
	First string `json:"first"`
	Last  string `json:"last"`
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
			http.ServeFile(w, r, fsRoot+"login.html")
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
		verifyUrl := (&url.URL{Scheme: "http", Host: r.Host, Path: "auth" + r.URL.Path, RawQuery: r.URL.RawQuery}).String()

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

		// Create user data JSON for cookie:
		userData := &UserCookieData{
			Email: q.Get("openid.ext1.value.email"),
			First: q.Get("openid.ext1.value.firstname"),
			Last:  q.Get("openid.ext1.value.lastname"),
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
