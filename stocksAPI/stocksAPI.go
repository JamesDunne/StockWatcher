package stocksAPI

// general stuff:
import (
	"math/big"
	"time"
)

// sqlite related imports:
import (
	"database/sql"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Our own packages:
import (
	"github.com/JamesDunne/StockWatcher/yql"
)

// Get the New York location for stock timezone:
var LocNY, _ = time.LoadLocation("America/New_York")

// ------------- public structures:

// Our API context struct:
type API struct {
	db              *sqlx.DB
	today           time.Time
	lastTradingDate time.Time
}

func (api *API) Today() time.Time           { return api.today }
func (api *API) LastTradingDate() time.Time { return api.lastTradingDate }

func (api *API) CurrentHour() time.Time { return time.Now().Truncate(time.Hour) }

type UserID int64

// ------------------------- API functions:

const dateFmt = "2006-01-02"
const sqliteFmt = "2006-01-02 15:04:05"

// Releases all API resources:
func (api *API) Close() {
	api.db.Close()
	api.db = nil
}

type User struct {
	UserID          UserID
	PrimaryEmail    string
	Name            string
	SecondaryEmails []string

	NotificationTimeout time.Duration
}

func (api *API) AddUser(user *User) (err error) {
	res, err := api.db.Exec(`insert into User (PrimaryEmail, Name, NotificationTimeout) values (?1,?2,?3)`, user.PrimaryEmail, user.Name, user.NotificationTimeout/time.Second)
	if err != nil {
		return
	}
	userID, err := res.LastInsertId()
	if err != nil {
		return
	}
	user.UserID = UserID(userID)

	if len(user.SecondaryEmails) > 0 {
		emails := make([][]interface{}, 0, len(user.SecondaryEmails))
		for _, e := range user.SecondaryEmails {
			emails = append(emails, []interface{}{e, user.UserID})
		}

		err = api.bulkInsert("UserEmail", []string{"Email", "UserID"}, emails)
		if err != nil {
			return
		}
	}
	return
}

func (api *API) GetUserByEmail(email string) (user *User, err error) {
	dbUser := new(struct {
		UserID              int64  `db:"UserID"`
		PrimaryEmail        string `db:"PrimaryEmail"`
		Name                string `db:"Name"`
		NotificationTimeout int    `db:"NotificationTimeout"`
	})

	// Get user by primary or secondary email:
	err = api.db.Get(dbUser, `select u.rowid as UserID, u.PrimaryEmail, u.Name, u.NotificationTimeout from User as u where u.PrimaryEmail = ?1
union all
select u.rowid as UserID, u.PrimaryEmail, u.Name, u.NotificationTimeout from User as u join UserEmail as ue on u.rowid = ue.UserID where ue.Email = ?1`, email)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return
	}

	// get emails:
	emails := make([]struct {
		Email string `db:"Email"`
	}, 0, 2)
	err = api.db.Select(&emails, `select ue.Email from UserEmail as ue where ue.UserID = ?1`, dbUser.UserID)
	if err == sql.ErrNoRows {
		emails = make([]struct {
			Email string `db:"Email"`
		}, 0, 2)
	} else if err != nil {
		return
	}

	user = &User{
		UserID:              UserID(dbUser.UserID),
		PrimaryEmail:        dbUser.PrimaryEmail,
		Name:                dbUser.Name,
		NotificationTimeout: time.Duration(dbUser.NotificationTimeout) * time.Second,
	}

	user.SecondaryEmails = make([]string, 0, len(emails))
	for _, e := range emails {
		user.SecondaryEmails = append(user.SecondaryEmails, e.Email)
	}

	return
}

// Add an owned stock for UserID:
func (api *API) AddOwnedStock(userID UserID, symbol string, buyDate string, buyPrice *big.Rat, shares int, stopPercent *big.Rat) (err error) {
	_, err = api.db.Exec(`insert or ignore into StockOwned (UserID, Symbol, BuyDate, IsEnabled, BuyPrice, Shares, StopPercent) values (?1,?2,?3,1,?4,?5,?6)`,
		int64(userID),
		symbol,
		buyDate,
		buyPrice.FloatString(2),
		shares,
		stopPercent.FloatString(2),
	)
	return
}

// Add a stock to watch for UserID:
func (api *API) AddWatchedStock(userID UserID, symbol string, startDate string, startPrice *big.Rat, stopPercent *big.Rat) (err error) {
	_, err = api.db.Exec(`insert or ignore into StockWatch (UserID, Symbol, IsEnabled, StartDate, StartPrice, StopPercent) values (?1,?2,1,?3,?4,?5)`,
		int64(userID),
		symbol,
		startDate,
		startPrice.FloatString(2),
		stopPercent.FloatString(2),
	)
	return
}

// A stock owned by UserID.
type OwnedStock struct {
	ID          int64
	UserID      UserID
	Symbol      string
	IsEnabled   bool
	BuyDate     time.Time
	BuyPrice    *big.Rat
	Shares      int
	StopPercent *big.Rat
}

// Gets all stocks owned by UserID:
func (api *API) GetOwnedStocksByUser(userID UserID) (owned []OwnedStock, err error) {
	// Anonymous structs are cool.
	rows := make([]struct {
		ID          int64  `db:"ID"`
		UserID      int64  `db:"UserID"`
		Symbol      string `db:"Symbol"`
		BuyDate     string `db:"BuyDate"`
		IsEnabled   int    `db:"IsEnabled"`
		BuyPrice    string `db:"BuyPrice"`
		Shares      int    `db:"Shares"`
		StopPercent string `db:"StopPercent"`
	}, 0, 6)

	err = api.db.Select(&rows, `
select rowid as ID, UserID, Symbol, BuyDate, IsEnabled, BuyPrice, Shares, StopPercent from StockOwned where UserID = ?1`, userID)
	if err != nil {
		return
	}

	// Copy raw DB rows into OwnedStock records:
	owned = make([]OwnedStock, 0, len(rows))
	for _, r := range rows {
		owned = append(owned, OwnedStock{
			ID:          r.ID,
			UserID:      UserID(r.UserID),
			Symbol:      r.Symbol,
			IsEnabled:   ToBool(r.IsEnabled),
			BuyDate:     TradeDate(r.BuyDate),
			BuyPrice:    ToRat(r.BuyPrice),
			Shares:      r.Shares,
			StopPercent: ToRat(r.StopPercent),
		})
	}

	return
}

// A stock watched by the UserID.
type WatchedStock struct {
	ID          int64
	UserID      UserID
	Symbol      string
	IsEnabled   bool
	StartDate   time.Time
	StartPrice  *big.Rat
	Shares      int
	StopPercent *big.Rat
}

// Gets all stocks watched by UserID:
func (api *API) GetWatchedStocksByUser(userID UserID) (watched []WatchedStock, err error) {
	// Anonymous structs are cool.
	rows := make([]struct {
		ID          int64  `db:"ID"`
		UserID      int64  `db:"UserID"`
		Symbol      string `db:"Symbol"`
		IsEnabled   int    `db:"IsEnabled"`
		StartDate   string `db:"StartDate"`
		StartPrice  string `db:"StartPrice"`
		Shares      int    `db:"Shares"`
		StopPercent string `db:"StopPercent"`
	}, 0, 6)

	err = api.db.Select(&rows, `
select rowid as ID, UserID, Symbol, IsEnabled, StartDate, StartPrice, StopPercent from StockWatch where UserID = ?1`, int64(userID))
	if err != nil {
		return
	}

	// Copy raw DB rows into WatchedStock records:
	watched = make([]WatchedStock, 0, len(rows))
	for _, r := range rows {
		watched = append(watched, WatchedStock{
			ID:          r.ID,
			UserID:      UserID(r.UserID),
			Symbol:      r.Symbol,
			IsEnabled:   ToBool(r.IsEnabled),
			StartDate:   TradeDate(r.StartDate),
			StartPrice:  ToRat(r.StartPrice),
			Shares:      r.Shares,
			StopPercent: ToRat(r.StopPercent),
		})
	}

	return
}

// Gets all actively tracked stock symbols (owned or watching):
func (api *API) GetAllTrackedSymbols() (symbols []string, err error) {
	rows := make([]struct {
		Symbol string `db:"Symbol"`
	}, 0, 4)

	err = api.db.Select(&rows, `
select distinct Symbol from (
	select Symbol from StockOwned where IsEnabled = 1
	union all
	select Symbol from StockWatch where IsEnabled = 1
)`)
	if err != nil {
		return
	}

	symbols = make([]string, 0, len(rows))
	for _, v := range rows {
		symbols = append(symbols, v.Symbol)
	}

	return
}

// Fetches historical data from Yahoo Finance into the database
func (api *API) RecordHistory(symbol string) (err error) {
	// Fetch earliest date of interest for symbol:
	var lastDate time.Time
	var lastTradeDay int64

	// Extract the last-fetched date from the db record, assuming NY time:
	ld, err := api.getScalars(`select h.Date, h.TradeDayIndex from StockHistory h where (h.Symbol = ?1) and (datetime(h.Date) = (select max(datetime(Date)) from StockHistory where Symbol = h.Symbol))`, symbol)
	if ld[0] != nil {
		lastDate = TruncDate(TradeDateTime(ld[0].(string)))
		lastTradeDay = ld[1].(int64)
	} else {
		// Find earliest date of interest for history:
		minBuyDateV, err := api.getScalar(`select min(datetime(BuyDate)) from StockOwned where Symbol = ?1`, symbol)
		if err != nil {
			return err
		}
		minStartDateV, err := api.getScalar(`select min(datetime(StartDate)) from StockWatch where Symbol = ?1`, symbol)
		if err != nil {
			return err
		}

		minDate := minNullTime(
			parseNullTime(sqliteFmt, minBuyDateV),
			parseNullTime(sqliteFmt, minStartDateV),
		)
		if minDate == nil {
			lastDate = api.lastTradingDate
		} else {
			lastDate = (*minDate)
		}

		// Take it back at least 42 weeks to get the 200-day moving average:
		lastDate = lastDate.Add(time.Duration(-42*7*24) * time.Hour)
		lastTradeDay = 0
	}

	// Do we need to fetch history?
	if !lastDate.Before(api.lastTradingDate) {
		return nil
	}

	// Fetch the historical data:
	hist, err := yql.GetHistory(symbol, lastDate, api.lastTradingDate)
	if err != nil {
		return err
	}

	// Bulk insert the historical data into the StockHistory table:
	rows := make([][]interface{}, 0, len(hist))
	for i, h := range hist {
		// Store dates as RFC3339 in the NYC timezone:
		date, err := time.ParseInLocation(dateFmt, h.Date, LocNY)
		if err != nil {
			return err
		}

		// Only record dates after last-fetched dates:
		if date.After(lastDate) {
			rows = append(rows, []interface{}{
				symbol,
				date.Format(time.RFC3339),
				lastTradeDay + int64(len(hist)-i),
				h.Close,
				h.Open,
				h.High,
				h.Low,
				h.Volume,
			})
		}
	}

	if len(rows) > 0 {
		err = api.bulkInsert("StockHistory", []string{"Symbol", "Date", "TradeDayIndex", "Closing", "Opening", "High", "Low", "Volume"}, rows)
		if err != nil {
			return
		}
	}

	return
}

// Calculates per-day trends and records them to the database.
func (api *API) RecordTrends(symbol string) (err error) {
	_, err = api.db.Exec(`
replace into StockHistoryTrend (Symbol, Date, Avg200Day, Avg50Day, SMAPercent)
select Symbol, Date, Avg200, Avg50, ((Avg50 / Avg200) - 1) * 100 as SMAPercent
from (
	select h.Symbol, h.Date
	     , (select avg(cast(Closing as real)) from StockHistory h0 where (h0.Symbol = h.Symbol) and (h0.TradeDayIndex >= (h.TradeDayIndex - 200))) as Avg200
	     , (select avg(cast(Closing as real)) from StockHistory h0 where (h0.Symbol = h.Symbol) and (h0.TradeDayIndex >= (h.TradeDayIndex - 50))) as Avg50
	from StockHistory h
	where (h.Symbol = ?1)
	  and (h.TradeDayIndex > 200)
)`, symbol)
	return
}

// Checks if the current hourly price has been fetched from Yahoo or not and fetches it into the StockHourly table if needed.
func (api *API) GetCurrentHourlyPrices(symbols ...string) (prices map[string]*big.Rat) {
	currHour := api.CurrentHour()

	toFetch := make([]string, 0, len(symbols))
	prices = make(map[string]*big.Rat)
	for _, symbol := range symbols {
		lastTimeStr, err := api.getScalar(`select max(datetime(DateTime)) from StockHourly where Symbol = ?1`, symbol)
		if err != nil {
			panic(err)
		}

		// Determine if we need to fetch from Yahoo or not:
		needFetch := false
		if lastTimeStr == nil {
			needFetch = true
		} else {
			lastHour := TradeSqliteDateTime(lastTimeStr.(string)).Truncate(time.Hour)
			if lastHour.Before(currHour) {
				needFetch = true
			}
		}

		// TODO(jsd): could break this out to separate single query with IN clause
		if !needFetch {
			currPriceStr, err := api.getScalar(`select Current from StockHourly where Symbol = ?1 and DateTime = ?2`, symbol, currHour.Format(time.RFC3339))
			if err != nil {
				panic(err)
			}
			prices[symbol] = ToRat(currPriceStr.(string))
			continue
		}

		// Add it to the list of symbols to be fetched from Yahoo:
		toFetch = append(toFetch, symbol)
	}

	// Get current prices from Yahoo:
	if len(toFetch) > 0 {
		quotes, err := yql.GetQuotes(toFetch...)
		if err != nil {
			panic(err)
		}

		for _, quote := range quotes {
			// Record the current hourly price:
			_, err = api.db.Exec(`replace into StockHourly (Symbol, DateTime, Current) values (?1, ?2, ?3)`,
				quote.Symbol,
				currHour.Format(time.RFC3339),
				quote.Price.FloatString(2),
			)
			if err != nil {
				panic(err)
			}

			// Fill in the price map:
			prices[quote.Symbol] = quote.Price
		}
	}

	return
}

// A stock owned by UserID with details.
type OwnedStockDetails struct {
	ID          int64
	UserID      int64
	Symbol      string
	IsEnabled   bool
	BuyDate     time.Time
	BuyPrice    *big.Rat
	Shares      int64
	StopPercent *big.Rat

	CurrPrice       *big.Rat
	TStopPrice      *big.Rat
	LastCloseDate   time.Time
	Avg200Day       float64
	Avg50Day        float64
	SMAPercent      float64
	GainLossPercent float64
	GainLossDollar  *big.Rat
}

// NOTE: Requires StockHistory, StockHistoryTrend, and StockHourly to be populated.
func (api *API) GetOwnedDetailsForUser(userID UserID) (details []OwnedStockDetails, err error) {
	// Anonymous structs are cool.
	rows := make([]struct {
		ID          int64  `db:"ID"`
		UserID      int64  `db:"UserID"`
		Symbol      string `db:"Symbol"`
		BuyDate     string `db:"BuyDate"`
		IsEnabled   int    `db:"IsEnabled"`
		BuyPrice    string `db:"BuyPrice"`
		Shares      int64  `db:"Shares"`
		StopPercent string `db:"StopPercent"`

		Current string `db:"Current"`

		Date         string  `db:"Date"`
		Avg200Day    float64 `db:"Avg200Day"`
		Avg50Day     float64 `db:"Avg50Day"`
		SMAPercent   float64 `db:"SMAPercent"`
		HighestClose float64 `db:"HighestClose"`
		LowestClose  float64 `db:"LowestClose"`
	}, 0, 6)

	err = api.db.Select(&rows, `
select ID, UserID, Symbol, BuyDate, IsEnabled, BuyPrice, Shares, StopPercent
     , Current
     , Date, Avg200Day, Avg50Day, SMAPercent
     , (select max(cast(h.Closing as real)) from StockHistory h where h.Symbol = o.Symbol) as HighestClose
     , (select min(cast(h.Closing as real)) from StockHistory h where h.Symbol = o.Symbol) as LowestClose
from (
	select o.rowid as ID, o.UserID, o.Symbol, o.BuyDate, o.IsEnabled, o.BuyPrice, o.Shares, o.StopPercent
	     , h.Current
	     , t.Date, t.Avg200Day, t.Avg50Day, t.SMAPercent
	from StockOwned o
	join StockHistoryTrend t on t.Symbol = o.Symbol
	join StockHourly h on h.Symbol = o.Symbol and h.DateTime = ?2
	where o.UserID = ?1
	order by t.Date desc limit 1
) o`, int64(userID), api.CurrentHour().Format(time.RFC3339))
	if err != nil {
		return
	}

	// Copy raw DB rows into OwnedStockDetails records:
	details = make([]OwnedStockDetails, 0, len(rows))
	for _, r := range rows {
		currPrice := ToRat(r.Current)
		d := OwnedStockDetails{
			ID:            r.ID,
			UserID:        r.UserID,
			Symbol:        r.Symbol,
			IsEnabled:     ToBool(r.IsEnabled),
			BuyDate:       TradeDate(r.BuyDate),
			BuyPrice:      ToRat(r.BuyPrice),
			Shares:        r.Shares,
			StopPercent:   ToRat(r.StopPercent),
			LastCloseDate: TradeDateTime(r.Date),

			Avg200Day:  r.Avg200Day,
			Avg50Day:   r.Avg50Day,
			SMAPercent: r.SMAPercent,
		}

		d.CurrPrice = currPrice
		if d.Shares > 0 {
			d.TStopPrice = new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Sub(ToRat("100"), d.StopPercent), ToRat("0.01"))), FloatToRat(r.HighestClose))
		} else {
			// Shorted:
			d.TStopPrice = new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Add(ToRat("100"), d.StopPercent), ToRat("0.01"))), FloatToRat(r.LowestClose))
		}

		// gain$ = (currPrice - buyPrice) * shares
		d.GainLossDollar = new(big.Rat).Mul(new(big.Rat).Sub(currPrice, d.BuyPrice), IntToRat(d.Shares))
		// gain% = ((currPrice - buyPrice) / buyPrice) * 100
		buyPriceFlt := RatToFloat(d.BuyPrice)
		d.GainLossPercent = ((RatToFloat(currPrice) - buyPriceFlt) * 100.0 / buyPriceFlt)

		details = append(details, d)
	}

	return
}
