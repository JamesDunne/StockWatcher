package stocksAPI

import (
	"fmt"
	"os"
	"testing"
	"time"
)

const tmpdb = "./tmp.db"

var api *API
var symbols []string
var err error

// These test functions run in sequential order as defined here:

func TestParseNullTime(t *testing.T) {
	fmt.Printf("null time: %+v\n", parseNullTime(dateFmt, nil))
	fmt.Printf("null time: %+v\n", parseNullTime(dateFmt, "2013-01-01"))
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

	fmt.Printf("user1: %+v\n", user)
}

func TestGetUserBySecondaryEmail(t *testing.T) {
	user, err := api.GetUserByEmail("test@example2.org")
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("user1: %+v\n", user)
}

func TestAddOwnedStock(t *testing.T) {
	err := api.AddOwnedStock(1, "MSFT", "2012-09-01", ToRat("40.00"), -10, ToRat("20.00"))
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestAddOwnedStock2(t *testing.T) {
	err := api.AddOwnedStock(1, "AAPL", "2012-09-01", ToRat("400.00"), +10, ToRat("20.00"))
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestGetOwnedStocks(t *testing.T) {
	stocks, err := api.GetOwnedStocksByUser(1)
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("owned stocks:    %+v\n", stocks)
}

func TestAddWatchedStock(t *testing.T) {
	err := api.AddWatchedStock(1, "AAPL", "2012-09-01", ToRat("400.00"), ToRat("20.00"))
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestAddWatchedStock2(t *testing.T) {
	err := api.AddWatchedStock(2, "AAPL", "2012-09-01", ToRat("400.00"), ToRat("20.00"))
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestGetWatchedStocks(t *testing.T) {
	stocks, err := api.GetWatchedStocksByUser(1)
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("watched stocks:  %+v\n", stocks)
}

func TestGetAllTrackedSymbols(t *testing.T) {
	symbols, err = api.GetAllTrackedSymbols()
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("tracked symbols: %+v\n", symbols)
}

func TestRecordHistory(t *testing.T) {
	for _, symbol := range symbols {
		fmt.Printf("recording history for %s...\n", symbol)
		err := api.RecordHistory(symbol)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRecordHistory2(t *testing.T) {
	for _, symbol := range symbols {
		fmt.Printf("recording history for %s...\n", symbol)
		err := api.RecordHistory(symbol)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRecordStats(t *testing.T) {
	for _, symbol := range symbols {
		fmt.Printf("recording stats for %s...\n", symbol)
		err := api.RecordStats(symbol)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestRecordStats2(t *testing.T) {
	for _, symbol := range symbols {
		fmt.Printf("recording stats for %s...\n", symbol)
		err := api.RecordStats(symbol)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestGetCurrentHourlyPrices(t *testing.T) {
	// Fetch multiple times in a row to test fetch from DB vs. fetch from Yahoo (and store to DB):
	for i := 1; i <= 10; i++ {
		prices := api.GetCurrentHourlyPrices("MSFT", "AAPL")
		fmt.Printf("prices [%d]: %+v\n", i, prices)
	}
}

func TestGetOwnedDetailsForUser(t *testing.T) {
	stocks, err := api.GetOwnedDetailsForUser(1)
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("detail stocks: %+v\n", stocks)
}

func TestGetOwnedDetailsForSymbol(t *testing.T) {
	stocks, err := api.GetOwnedDetailsForSymbol("MSFT")
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("detail stocks: %+v\n", stocks)
}

func TestGetWatchedDetailsForUser(t *testing.T) {
	stocks, err := api.GetWatchedDetailsForUser(1)
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("detail stocks: %+v\n", stocks)
}

func TestGetWatchedDetailsForSymbol(t *testing.T) {
	stocks, err := api.GetWatchedDetailsForSymbol("AAPL")
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("detail stocks: %+v\n", stocks)
}
