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

// Add an owned stock for UserID:
func (api *API) AddOwned(userID UserID, symbol string, buyDate string, buyPrice *big.Rat, shares int, stopPercent *big.Rat) (err error) {
	_, err = api.db.Exec(`insert or ignore into StockOwned (UserID, Symbol, BuyDate, IsEnabled, BuyPrice, Shares, TStopPercent) values (?1,?2,?3,1,?4,?5,?6)`,
		int64(userID),
		symbol,
		buyDate,
		buyPrice.FloatString(2),
		shares,
		stopPercent.FloatString(2),
	)
	return
}

// Removes an owned stock:
func (api *API) RemoveOwned(ownedID OwnedID) (err error) {
	_, err = api.db.Exec(`delete from StockOwned where rowid = ?1`, int64(ownedID))
	return
}

// A stock owned by UserID.
type OwnedStock struct {
	OwnedID      OwnedID
	UserID       UserID
	Symbol       string
	IsEnabled    bool
	BuyDate      time.Time
	BuyPrice     *big.Rat
	Shares       int
	TStopPercent *big.Rat
}

// Gets all stocks owned by UserID:
func (api *API) GetOwnedByUser(userID UserID) (owned []OwnedStock, err error) {
	// Anonymous structs are cool.
	rows := make([]struct {
		ID           int64  `db:"ID"`
		UserID       int64  `db:"UserID"`
		Symbol       string `db:"Symbol"`
		BuyDate      string `db:"BuyDate"`
		IsEnabled    int    `db:"IsEnabled"`
		BuyPrice     string `db:"BuyPrice"`
		Shares       int    `db:"Shares"`
		TStopPercent string `db:"TStopPercent"`
	}, 0, 6)

	err = api.db.Select(&rows, `
select rowid as ID, UserID, Symbol, BuyDate, IsEnabled, BuyPrice, Shares, TStopPercent from StockOwned where UserID = ?1`, userID)
	if err != nil {
		return
	}

	// Copy raw DB rows into OwnedStock records:
	owned = make([]OwnedStock, 0, len(rows))
	for _, r := range rows {
		owned = append(owned, OwnedStock{
			OwnedID:      OwnedID(r.ID),
			UserID:       UserID(r.UserID),
			Symbol:       r.Symbol,
			IsEnabled:    ToBool(r.IsEnabled),
			BuyDate:      TradeDate(r.BuyDate),
			BuyPrice:     ToRat(r.BuyPrice),
			Shares:       r.Shares,
			TStopPercent: ToRat(r.TStopPercent),
		})
	}

	return
}

type dbOwnedDetail struct {
	ID                  int64          `db:"ID"`
	UserID              int64          `db:"UserID"`
	Symbol              string         `db:"Symbol"`
	BuyDate             string         `db:"BuyDate"`
	IsEnabled           int            `db:"IsEnabled"`
	BuyPrice            string         `db:"BuyPrice"`
	Shares              int64          `db:"Shares"`
	TStopPercent        string         `db:"TStopPercent"`
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
type OwnedDetails struct {
	OwnedID             OwnedID
	UserID              UserID
	Symbol              string
	IsEnabled           bool
	BuyDate             time.Time
	BuyPrice            *big.Rat
	Shares              int64
	TStopPercent        *big.Rat
	LastTStopNotifyTime *time.Time

	CurrHour  time.Time
	CurrPrice *big.Rat

	// Calculated values:

	LastCloseDate   time.Time
	TStopPrice      *big.Rat
	Avg200Day       float64
	Avg50Day        float64
	SMAPercent      float64
	GainLossPercent float64
	GainLossDollar  *big.Rat
}

func projectOwnedDetails(rows []dbOwnedDetail) (details []OwnedDetails, err error) {
	// Copy raw DB rows into OwnedDetails records:
	details = make([]OwnedDetails, 0, len(rows))
	for _, r := range rows {
		currPrice := ToRat(r.CurrPrice)
		d := OwnedDetails{
			OwnedID:             OwnedID(r.ID),
			UserID:              UserID(r.UserID),
			Symbol:              r.Symbol,
			IsEnabled:           ToBool(r.IsEnabled),
			BuyDate:             TradeDate(r.BuyDate),
			BuyPrice:            ToRat(r.BuyPrice),
			Shares:              r.Shares,
			TStopPercent:        ToRat(r.TStopPercent),
			LastCloseDate:       TradeDateTime(r.Date),
			LastTStopNotifyTime: sqliteNullTime(time.RFC3339, r.LastTStopNotifyTime),

			CurrPrice: currPrice,
			CurrHour:  TradeDateTime(r.CurrHour),

			Avg200Day:  r.Avg200Day,
			Avg50Day:   r.Avg50Day,
			SMAPercent: r.SMAPercent,
		}

		buyPriceFlt := RatToFloat(d.BuyPrice)

		if d.Shares >= 0 {
			// Owned:

			// ((100 - stopPercent) * 0.01) * highestClose
			d.TStopPrice = new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Sub(ToRat("100"), d.TStopPercent), ToRat("0.01"))), FloatToRat(r.HighestClose))

			// gain% = ((currPrice / buyPrice) - 1) * 100
			d.GainLossPercent = (((RatToFloat(currPrice) / buyPriceFlt) - 1.0) * 100.0)
		} else {
			// Shorted:

			// ((100 + stopPercent) * 0.01) * lowestClose
			d.TStopPrice = new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Add(ToRat("100"), d.TStopPercent), ToRat("0.01"))), FloatToRat(r.LowestClose))

			// gain% = -((buyPrice / currPrice) - 1) * 100
			d.GainLossPercent = (((buyPriceFlt / RatToFloat(currPrice)) - 1.0) * 100.0)
		}

		// gain$ = (currPrice - buyPrice) * shares
		d.GainLossDollar = new(big.Rat).Mul(new(big.Rat).Sub(currPrice, d.BuyPrice), IntToRat(d.Shares))

		// Add to list:
		details = append(details, d)
	}

	return
}

// NOTE: Requires StockHistory, StockStats, and StockHourly to be populated.
func (api *API) GetOwnedDetailsForUser(userID UserID) (details []OwnedDetails, err error) {
	rows := make([]dbOwnedDetail, 0, 6)

	err = api.db.Select(&rows, `
select ID, UserID, Symbol, BuyDate, IsEnabled, BuyPrice, Shares, TStopPercent, LastTStopNotifyTime, CurrPrice, CurrHour, Date, Avg200Day, Avg50Day, SMAPercent, HighestClose, LowestClose
from StockOwnedDetail o
where (o.UserID = ?1)
  and (datetime(o.CurrHour) = (select max(datetime(h.DateTime)) from StockHourly h where h.Symbol = o.Symbol))
order by o.ID asc`, int64(userID))
	if err != nil {
		return
	}

	details, err = projectOwnedDetails(rows)
	return
}

// NOTE: Requires StockHistory, StockStats, and StockHourly to be populated.
func (api *API) GetOwnedDetailsForSymbol(symbol string) (details []OwnedDetails, err error) {
	rows := make([]dbOwnedDetail, 0, 6)

	err = api.db.Select(&rows, `
select ID, UserID, Symbol, BuyDate, IsEnabled, BuyPrice, Shares, TStopPercent, LastTStopNotifyTime, CurrPrice, CurrHour, Date, Avg200Day, Avg50Day, SMAPercent, HighestClose, LowestClose
from StockOwnedDetail o
where (o.Symbol = ?1)
  and (datetime(o.CurrHour) = (select max(datetime(h.DateTime)) from StockHourly h where h.Symbol = o.Symbol))
order by o.ID asc`, symbol)
	if err != nil {
		return
	}

	details, err = projectOwnedDetails(rows)
	return
}

func (api *API) EnableOwned(ownedID OwnedID) (err error) {
	_, err = api.db.Exec(`update StockOwned set IsEnabled = 1 where rowid = ?1`, int64(ownedID))
	return
}

func (api *API) DisableOwned(ownedID OwnedID) (err error) {
	_, err = api.db.Exec(`update StockOwned set IsEnabled = 0 where rowid = ?1`, int64(ownedID))
	return
}

func (api *API) UpdateOwnedLastNotifyTime(ownedID OwnedID, lastNotifyDate time.Time) (err error) {
	_, err = api.db.Exec(`update StockOwned set LastTStopNotifyTime = ?2 where rowid = ?1`, int64(ownedID), lastNotifyDate.Format(time.RFC3339))
	return
}
