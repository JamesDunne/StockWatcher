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

// Converts a string into a `*big.Rat` which is an arbitrary precision rational number stored in decimal format
func ToRat(v string) *big.Rat {
	rat := new(big.Rat)
	rat.SetString(v)
	return rat
}

// remove the time component of a datetime to get just a date at 00:00:00
func TruncDate(t time.Time) time.Time {
	hour, min, sec := t.Clock()
	nano := t.Nanosecond()

	d := time.Duration(0) - (time.Duration(nano) + time.Duration(sec)*time.Second + time.Duration(min)*time.Minute + time.Duration(hour)*time.Hour)
	return t.Add(d)
}

// parses the date/time in RFC3339 format, assuming NY timezone:
func TradeDateTime(str string) (t time.Time, err error) {
	return time.ParseInLocation(time.RFC3339, str, LocNY)
}

// Check if the date is on a weekend:
func IsWeekend(date time.Time) bool {
	return date.Weekday() == 0 || date.Weekday() == 6
}

// ------------------------- API functions:

const dateFmt = "2006-01-02"
const sqliteFmt = "2006-01-02 15:04:05"

// Releases all API resources:
func (api *API) Close() {
	api.db.Close()
	api.db = nil
}

type User struct {
	UserID          int
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
	user.UserID = int(userID)

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
		UserID              int    `db:"UserID"`
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
		UserID:              dbUser.UserID,
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

// Add a stock owned by UserID:
func (api *API) AddOwnedStock(userID int, symbol string, buyDate string, buyPrice *big.Rat, shares int, stopPercent *big.Rat) (err error) {
	_, err = api.db.Exec(`insert or ignore into StockOwned (UserID, Symbol, BuyDate, IsEnabled, BuyPrice, Shares, StopPercent) values (?1,?2,?3,1,?4,?5,?6)`,
		userID,
		symbol,
		buyDate,
		buyPrice.FloatString(2),
		shares,
		stopPercent.FloatString(2),
	)
	return
}

// Gets all actively tracked stock symbols:
func (api *API) GetTrackedSymbols() (symbols []string, err error) {
	rows := make([]struct {
		Symbol string `db:"Symbol"`
	}, 0, 4)

	err = api.db.Select(&rows, `
select Symbol from StockOwned where IsEnabled = 1
union all
select Symbol from StockWatch where IsEnabled = 1`)
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

	// Extract the last-fetched date from the db record, assuming NY time:
	ld, err := api.getScalar(`select h.Date from StockHistory h where (h.Symbol = ?1) and (datetime(h.Date) = (select max(datetime(Date)) from StockHistory where Symbol = h.Symbol))`, symbol)
	if ld != nil {
		tmp, err := TradeDateTime(ld.(string))
		if err != nil {
			return err
		}
		lastDate = TruncDate(tmp)
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

		// Take it back at least 200 days to get the 200-day moving average:
		lastDate = lastDate.Add(time.Duration(-200*24) * time.Hour)
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
	for _, h := range hist {
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
				h.Close,
				h.Open,
				h.High,
				h.Low,
				h.Volume,
			})
		}
	}

	if len(rows) > 0 {
		err = api.bulkInsert("StockHistory", []string{"Symbol", "Date", "Closing", "Opening", "High", "Low", "Volume"}, rows)
		if err != nil {
			return
		}
	}

	return
}

func (api *API) RecordTrends(symbol string) (err error) {
	_, err = api.db.Exec(`
replace into StockHistoryTrend (Symbol, Date, Avg200Day, Avg50Day, SMAPercent)
select Symbol, Date, Avg200, Avg50, ((Avg200 / Avg50) - 1) * 100 as SMAPercent
from (
	select h.Symbol, h.Date
	     , (select avg(cast(Closing as real)) from StockHistory h0 where (h0.Symbol = h.Symbol) and (datetime(h0.Date) between datetime(h.Date, '-200 days') and datetime(h.Date))) as Avg200
	     , (select avg(cast(Closing as real)) from StockHistory h0 where (h0.Symbol = h.Symbol) and (datetime(h0.Date) between datetime(h.Date, '-50 days') and datetime(h.Date))) as Avg50
	from StockHistory h
	where (h.Symbol = ?1)
	  and (datetime(h.Date) >= datetime((select min(datetime(Date)) from StockHistory where Symbol = h.Symbol), '+200 days'))
)`, symbol)
	return
}
