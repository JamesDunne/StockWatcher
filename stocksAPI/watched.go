package stocksAPI

// general stuff:
import (
	"math/big"
	"time"
)

// sqlite related imports:
import (
	"database/sql"
	//"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Add a stock to watch for UserID:
func (api *API) AddWatched(userID UserID, symbol string, startDate string, startPrice *big.Rat, stopPercent *big.Rat) (err error) {
	_, err = api.db.Exec(`insert or ignore into StockWatched (UserID, Symbol, IsEnabled, StartDate, StartPrice, TStopPercent) values (?1,?2,1,?3,?4,?5)`,
		int64(userID),
		symbol,
		startDate,
		startPrice.FloatString(2),
		stopPercent.FloatString(2),
	)
	return
}

// Removes a watched stock:
func (api *API) RemoveWatched(watchedID WatchedID) (err error) {
	_, err = api.db.Exec(`delete from StockWatched where rowid = ?1`, int64(watchedID))
	return
}

// A stock watched by the UserID.
type WatchedStock struct {
	WatchedID    WatchedID
	UserID       UserID
	Symbol       string
	IsEnabled    bool
	StartDate    time.Time
	StartPrice   *big.Rat
	TStopPercent *big.Rat
}

// Gets all stocks watched by UserID:
func (api *API) GetWatchedByUser(userID UserID) (watched []WatchedStock, err error) {
	// Anonymous structs are cool.
	rows := make([]struct {
		ID           int64  `db:"ID"`
		UserID       int64  `db:"UserID"`
		Symbol       string `db:"Symbol"`
		IsEnabled    int    `db:"IsEnabled"`
		StartDate    string `db:"StartDate"`
		StartPrice   string `db:"StartPrice"`
		TStopPercent string `db:"TStopPercent"`
	}, 0, 6)

	err = api.db.Select(&rows, `
select rowid as ID, UserID, Symbol, IsEnabled, StartDate, StartPrice, TStopPercent from StockWatched where UserID = ?1`, int64(userID))
	if err != nil {
		return
	}

	// Copy raw DB rows into WatchedStock records:
	watched = make([]WatchedStock, 0, len(rows))
	for _, r := range rows {
		watched = append(watched, WatchedStock{
			WatchedID:    WatchedID(r.ID),
			UserID:       UserID(r.UserID),
			Symbol:       r.Symbol,
			IsEnabled:    ToBool(r.IsEnabled),
			StartDate:    TradeDate(r.StartDate),
			StartPrice:   ToRat(r.StartPrice),
			TStopPercent: ToRat(r.TStopPercent),
		})
	}

	return
}

type dbWatchedDetail struct {
	ID                  int64          `db:"ID"`
	UserID              int64          `db:"UserID"`
	Symbol              string         `db:"Symbol"`
	IsEnabled           int            `db:"IsEnabled"`
	TStopPercent        string         `db:"TStopPercent"`
	StartDate           string         `db:"StartDate"`
	StartPrice          string         `db:"StartPrice"`
	LastTStopNotifyTime sql.NullString `db:"LastTStopNotifyTime"`

	CurrPrice string `db:"CurrPrice"`
	CurrHour  string `db:"CurrHour"`

	Date         string  `db:"Date"`
	Avg200Day    float64 `db:"Avg200Day"`
	Avg50Day     float64 `db:"Avg50Day"`
	SMAPercent   float64 `db:"SMAPercent"`
	HighestClose float64 `db:"HighestClose"`
	LowestClose  float64 `db:"LowestClose"`
}

// A stock owned by UserID with calculated details.
type WatchedDetails struct {
	WatchedID           WatchedID
	UserID              UserID
	Symbol              string
	IsEnabled           bool
	StartDate           time.Time
	StartPrice          *big.Rat
	TStopPercent        *big.Rat
	LastTStopNotifyTime *time.Time

	CurrHour  time.Time
	CurrPrice *big.Rat

	// Calculated values:

	LastCloseDate time.Time
	TStopPrice    *big.Rat
	Avg200Day     float64
	Avg50Day      float64
	SMAPercent    float64
}

func projectWatchedDetails(rows []dbWatchedDetail) (details []WatchedDetails, err error) {
	// Copy raw DB rows into WatchedDetails records:
	details = make([]WatchedDetails, 0, len(rows))
	for _, r := range rows {
		currPrice := ToRat(r.CurrPrice)
		d := WatchedDetails{
			WatchedID:           WatchedID(r.ID),
			UserID:              UserID(r.UserID),
			Symbol:              r.Symbol,
			IsEnabled:           ToBool(r.IsEnabled),
			StartDate:           TradeDate(r.StartDate),
			StartPrice:          ToRat(r.StartPrice),
			TStopPercent:        ToRat(r.TStopPercent),
			LastTStopNotifyTime: sqliteNullTime(time.RFC3339, r.LastTStopNotifyTime),

			CurrPrice: currPrice,
			CurrHour:  TradeDateTime(r.CurrHour),

			LastCloseDate: TradeDateTime(r.Date),
			Avg200Day:     r.Avg200Day,
			Avg50Day:      r.Avg50Day,
			SMAPercent:    r.SMAPercent,
		}

		//if d.Shares > 0 {
		// ((100 - stopPercent) * 0.01) * highestClose
		d.TStopPrice = new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Sub(ToRat("100"), d.TStopPercent), ToRat("0.01"))), FloatToRat(r.HighestClose))
		//} else {
		//	// Shorted:
		//	// ((100 + stopPercent) * 0.01) * lowestClose
		//	d.TStopPrice = new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Add(ToRat("100"), d.TStopPercent), ToRat("0.01"))), FloatToRat(r.LowestClose))
		//}

		// Add to list:
		details = append(details, d)
	}

	return
}

// NOTE: Requires StockHistory, StockStats, and StockHourly to be populated.
func (api *API) GetWatchedDetailsForUser(userID UserID) (details []WatchedDetails, err error) {
	rows := make([]dbWatchedDetail, 0, 6)

	err = api.db.Select(&rows, `
select ID, UserID, Symbol, IsEnabled, StartDate, StartPrice, TStopPercent, LastTStopNotifyTime, CurrPrice, CurrHour, Date, Avg200Day, Avg50Day, SMAPercent, HighestClose, LowestClose
from StockWatchedDetail o
where (o.UserID = ?1)
  and (datetime(o.CurrHour) = (select max(datetime(h.DateTime)) from StockHourly h where h.Symbol = o.Symbol))
order by o.ID asc`, int64(userID))
	if err != nil {
		return
	}

	details, err = projectWatchedDetails(rows)
	return
}

// NOTE: Requires StockHistory, StockStats, and StockHourly to be populated.
func (api *API) GetWatchedDetailsForSymbol(symbol string) (details []WatchedDetails, err error) {
	rows := make([]dbWatchedDetail, 0, 6)

	err = api.db.Select(&rows, `
select ID, UserID, Symbol, IsEnabled, StartDate, StartPrice, TStopPercent, LastTStopNotifyTime, CurrPrice, CurrHour, Date, Avg200Day, Avg50Day, SMAPercent, HighestClose, LowestClose
from StockWatchedDetail o
where (o.Symbol = ?1)
  and (datetime(o.CurrHour) = (select max(datetime(h.DateTime)) from StockHourly h where h.Symbol = o.Symbol))
order by o.ID asc`, symbol)
	if err != nil {
		return
	}

	details, err = projectWatchedDetails(rows)
	return
}

func (api *API) EnableWatched(ownedID WatchedID) (err error) {
	_, err = api.db.Exec(`update StockWatched set IsEnabled = 1 where rowid = ?1`, int64(ownedID))
	return
}

func (api *API) DisableWatched(ownedID WatchedID) (err error) {
	_, err = api.db.Exec(`update StockWatched set IsEnabled = 0 where rowid = ?1`, int64(ownedID))
	return
}

func (api *API) UpdateWatchedLastNotifyTime(ownedID WatchedID, lastNotifyDate time.Time) (err error) {
	_, err = api.db.Exec(`update StockWatched set LastTStopNotifyTime = ?2 where rowid = ?1`, int64(ownedID), lastNotifyDate.Format(time.RFC3339))
	return
}
