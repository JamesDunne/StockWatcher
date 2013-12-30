package stocks

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
		Name:                "Test User",
		NotificationTimeout: time.Duration(24) * time.Hour,
		Emails: []UserEmail{
			UserEmail{Email: "test@example.org", IsPrimary: true},
			UserEmail{Email: "test@example2.org", IsPrimary: false},
			UserEmail{Email: "test@example3.org", IsPrimary: false},
		},
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

func TestAddStock(t *testing.T) {
	s := Stock{
		UserID:    UserID(1),
		Symbol:    "MSFT",
		BuyDate:   ToDateTime(dateFmt, "2012-09-03"),
		BuyPrice:  ToDecimal("40.00"),
		Shares:    int64(-10),
		IsWatched: false,

		TStopPercent:  ToNullDecimal("20.00"),
		BuyStopPrice:  DecimalNull,
		SellStopPrice: DecimalNull,
	}
	err := api.AddStock(&s)
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestAddStock2(t *testing.T) {
	s := Stock{
		UserID:    UserID(1),
		Symbol:    "AAPL",
		BuyDate:   ToDateTime(dateFmt, "2012-09-03"),
		BuyPrice:  ToDecimal("400.00"),
		Shares:    int64(+10),
		IsWatched: false,

		TStopPercent:  ToNullDecimal("20.00"),
		BuyStopPrice:  DecimalNull,
		SellStopPrice: DecimalNull,
	}
	err := api.AddStock(&s)
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestAddWatchedStock(t *testing.T) {
	s := Stock{
		UserID:    UserID(1),
		Symbol:    "MSFT",
		BuyDate:   ToDateTime(dateFmt, "2012-09-03"),
		BuyPrice:  ToDecimal("40.00"),
		Shares:    int64(0),
		IsWatched: true,

		TStopPercent:  ToNullDecimal("20.00"),
		BuyStopPrice:  DecimalNull,
		SellStopPrice: DecimalNull,
	}
	err := api.AddStock(&s)
	if err != nil {
		t.Fatal(err)
		return
	}
}

func TestAddWatchedStock2(t *testing.T) {
	s := Stock{
		UserID:    UserID(1),
		Symbol:    "AAPL",
		BuyDate:   ToDateTime(dateFmt, "2012-09-03"),
		BuyPrice:  ToDecimal("400.00"),
		Shares:    int64(0),
		IsWatched: true,

		TStopPercent:  ToNullDecimal("20.00"),
		BuyStopPrice:  DecimalNull,
		SellStopPrice: DecimalNull,
	}
	err := api.AddStock(&s)
	if err != nil {
		t.Fatal(err)
		return
	}
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

func TestGetStockDetailsForUser(t *testing.T) {
	stocks, err := api.GetStockDetailsForUser(1)
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("detail stocks: %+v\n", stocks)
}

func TestGetStockDetailsForSymbol(t *testing.T) {
	stocks, err := api.GetStockDetailsForSymbol("MSFT")
	if err != nil {
		t.Fatal(err)
		return
	}

	fmt.Printf("detail stocks: %+v\n", stocks)
}
