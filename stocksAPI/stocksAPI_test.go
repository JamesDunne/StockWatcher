package stocksAPI

import (
	"fmt"
	"os"
	"testing"
	"time"
)

const tmpdb = "./tmp.db"

var api *API

// These test functions run in sequential order as defined here:

func TestParseNullTime(t *testing.T) {
	fmt.Printf("null time: %v\n", parseNullTime(dateFmt, nil))
	fmt.Printf("null time: %v\n", parseNullTime(dateFmt, "2013-01-01"))
}

func TestTruncDate(t *testing.T) {
	// Get the New York location for stock timezone:
	nyLoc, _ := time.LoadLocation("America/New_York")

	today := TruncDate(time.Now().In(nyLoc))
	yesterday := today.Add(time.Hour * time.Duration(-24))

	fmt.Println(today)
	fmt.Println(yesterday)
}

func TestNewAPI(t *testing.T) {
	os.Remove(tmpdb)
	var err error
	api, err = NewAPI(tmpdb)
	if err != nil {
		t.Fatal(err)
		return
	}
	return
}

func TestAddUser(t *testing.T) {
	user := &User{
		PrimaryEmail:        "test@example.org",
		Name:                "Test User",
		NotificationTimeout: time.Duration(24) * time.Hour,
		SecondaryEmails:     []string{"test@example2.org", "test@example3.org"},
	}
	err := api.AddUser(user)
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestGetUserByEmail(t *testing.T) {
	user, err := api.GetUserByEmail("test@example.org")
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("user: %v\n", user)
}

func TestGetUserBySecondaryEmail(t *testing.T) {
	user, err := api.GetUserByEmail("test@example2.org")
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("user: %v\n", user)
}

func TestAddOwnedStock(t *testing.T) {
	err := api.AddOwnedStock(1, "MSFT", "2012-09-01", ToRat("40.00"), +10, ToRat("20.00"))
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestGetTrackedSymbols(t *testing.T) {
	symbols, err := api.GetTrackedSymbols()
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("symbols: %v\n", symbols)
}

func TestRecordHistory(t *testing.T) {
	err := api.RecordHistory("MSFT")
	if err != nil {
		t.Fatal(err)
	}
}

func TestRecordTrends(t *testing.T) {
	err := api.RecordTrends("MSFT")
	if err != nil {
		t.Fatal(err)
	}
}
