package stocks

// general stuff:
import (
	"fmt"
	"math/big"
	"time"
)

// sqlite related imports:
import (
	"database/sql"
	//"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// A stock owned/watched by UserID.
type Stock struct {
	StockID   StockID
	UserID    UserID
	Symbol    string
	BuyDate   DateTime
	BuyPrice  Decimal
	Shares    int64
	IsWatched bool // false = owned, true = watched

	TStopPercent     NullDecimal
	BuyStopPrice     NullDecimal
	SellStopPrice    NullDecimal
	RisePercent      NullDecimal
	FallPercent      NullDecimal
	NotifyTStop      bool
	NotifyBuyStop    bool
	NotifySellStop   bool
	NotifyRise       bool
	NotifyFall       bool
	NotifyBullBear   bool
	LastTimeTStop    NullDateTime
	LastTimeBuyStop  NullDateTime
	LastTimeSellStop NullDateTime
	LastTimeRise     NullDateTime
	LastTimeFall     NullDateTime
	LastTimeBullBear NullDateTime
}

type Detail struct {
	CurrPrice       NullDecimal
	CurrHour        NullDateTime
	FetchedDateTime NullDateTime

	N1CloseDate  NullDateTime
	N1ClosePrice NullDecimal
	N1SMAPercent NullFloat64
	N1Avg200Day  NullFloat64
	N1Avg50Day   NullFloat64

	N2CloseDate  NullDateTime
	N2ClosePrice NullDecimal
	N2SMAPercent NullFloat64

	TStopPrice      NullDecimal
	GainLossPercent NullFloat64
	GainLossDollar  NullDecimal
}

// A stock with calculated stats:
type StockDetail struct {
	Stock  Stock
	Detail Detail
}

type dbStock struct {
	StockID   int64  `db:"StockID"`
	UserID    int64  `db:"UserID"`
	Symbol    string `db:"Symbol"`
	BuyDate   string `db:"BuyDate"`
	BuyPrice  string `db:"BuyPrice"`
	Shares    int64  `db:"Shares"`
	IsWatched int64  `db:"IsWatched"`

	TStopPercent     sql.NullString `db:"TStopPercent"`
	BuyStopPrice     sql.NullString `db:"BuyStopPrice"`
	SellStopPrice    sql.NullString `db:"SellStopPrice"`
	RisePercent      sql.NullString `db:"RisePercent"`
	FallPercent      sql.NullString `db:"FallPercent"`
	NotifyTStop      int64          `db:"NotifyTStop"`
	NotifyBuyStop    int64          `db:"NotifyBuyStop"`
	NotifySellStop   int64          `db:"NotifySellStop"`
	NotifyRise       int64          `db:"NotifyRise"`
	NotifyFall       int64          `db:"NotifyFall"`
	NotifyBullBear   int64          `db:"NotifyBullBear"`
	LastTimeTStop    sql.NullString `db:"LastTimeTStop"`
	LastTimeBuyStop  sql.NullString `db:"LastTimeBuyStop"`
	LastTimeSellStop sql.NullString `db:"LastTimeSellStop"`
	LastTimeRise     sql.NullString `db:"LastTimeRise"`
	LastTimeFall     sql.NullString `db:"LastTimeFall"`
	LastTimeBullBear sql.NullString `db:"LastTimeBullBear"`
}

// DB representation of a stock with calculated stats:
type dbDetail struct {
	// Include all fields from dbStock:
	dbStock

	CurrPrice       sql.NullString `db:"CurrPrice"`
	CurrHour        sql.NullString `db:"CurrHour"`
	FetchedDateTime sql.NullString `db:"FetchedDateTime"`

	N1CloseDate  sql.NullString  `db:"N1CloseDate"`
	N1ClosePrice sql.NullString  `db:"N1ClosePrice"`
	N1SMAPercent sql.NullFloat64 `db:"N1SMAPercent"`
	N1Avg200Day  sql.NullFloat64 `db:"N1Avg200Day"`
	N1Avg50Day   sql.NullFloat64 `db:"N1Avg50Day"`

	N2CloseDate  sql.NullString  `db:"N2CloseDate"`
	N2ClosePrice sql.NullString  `db:"N2ClosePrice"`
	N2SMAPercent sql.NullFloat64 `db:"N2SMAPercent"`

	HighestClose sql.NullFloat64 `db:"HighestClose"`
	LowestClose  sql.NullFloat64 `db:"LowestClose"`
}

// Add a stock for UserID:
func (api *API) AddStock(s *Stock) (err error) {
	if s == nil {
		return fmt.Errorf("s cannot be nil for AddStock")
	}

	// Insert the Stock record:
	res, err := api.db.Exec(`
insert into Stock (`+stockCols+`)
    values (?1,?2,?3,?4,?5,?6,?7,?8,?9,?10,?11,?12,?13,?14,?15,?16,?17,?18,?19,?20,?21,?22,?23)`,
		int64(s.UserID),
		s.Symbol,
		toDbDateTime(s.BuyDate),
		toDbDecimal(s.BuyPrice, 2),
		s.Shares,
		toDbBool(s.IsWatched),
		toDbNullDecimal(s.TStopPercent, 2),
		toDbNullDecimal(s.BuyStopPrice, 2),
		toDbNullDecimal(s.SellStopPrice, 2),
		toDbNullDecimal(s.RisePercent, 2),
		toDbNullDecimal(s.FallPercent, 2),
		toDbBool(s.NotifyTStop),
		toDbBool(s.NotifyBuyStop),
		toDbBool(s.NotifySellStop),
		toDbBool(s.NotifyRise),
		toDbBool(s.NotifyFall),
		toDbBool(s.NotifyBullBear),
		toDbNullDateTime(time.RFC3339, s.LastTimeTStop),
		toDbNullDateTime(time.RFC3339, s.LastTimeBuyStop),
		toDbNullDateTime(time.RFC3339, s.LastTimeSellStop),
		toDbNullDateTime(time.RFC3339, s.LastTimeRise),
		toDbNullDateTime(time.RFC3339, s.LastTimeFall),
		toDbNullDateTime(time.RFC3339, s.LastTimeBullBear),
	)
	if err != nil {
		s.StockID = StockID(0)
		return err
	}

	// Get last inserted ID:
	id, err := res.LastInsertId()
	if err != nil {
		s.StockID = StockID(0)
		return err
	}

	// Set StockID:
	s.StockID = StockID(id)
	return nil
}

// Gets a stock by ID:
func (api *API) GetStock(stockID StockID) (s *Stock, err error) {
	r := dbStock{}
	err = api.db.Get(&r, `select StockID,`+stockCols+` from Stock where StockID = ?1`, int64(stockID))
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	s = &Stock{
		StockID:   StockID(r.StockID),
		UserID:    UserID(r.UserID),
		Symbol:    r.Symbol,
		BuyDate:   fromDbDateTime(dateFmt, r.BuyDate),
		BuyPrice:  fromDbDecimal(r.BuyPrice),
		Shares:    r.Shares,
		IsWatched: fromDbBool(r.IsWatched),

		TStopPercent:     fromDbNullDecimal(r.TStopPercent),
		BuyStopPrice:     fromDbNullDecimal(r.BuyStopPrice),
		SellStopPrice:    fromDbNullDecimal(r.SellStopPrice),
		RisePercent:      fromDbNullDecimal(r.RisePercent),
		FallPercent:      fromDbNullDecimal(r.FallPercent),
		NotifyTStop:      fromDbBool(r.NotifyTStop),
		NotifyBuyStop:    fromDbBool(r.NotifyBuyStop),
		NotifySellStop:   fromDbBool(r.NotifySellStop),
		NotifyRise:       fromDbBool(r.NotifyRise),
		NotifyFall:       fromDbBool(r.NotifyFall),
		NotifyBullBear:   fromDbBool(r.NotifyBullBear),
		LastTimeTStop:    fromDbNullDateTime(time.RFC3339, r.LastTimeTStop),
		LastTimeBuyStop:  fromDbNullDateTime(time.RFC3339, r.LastTimeBuyStop),
		LastTimeSellStop: fromDbNullDateTime(time.RFC3339, r.LastTimeSellStop),
		LastTimeRise:     fromDbNullDateTime(time.RFC3339, r.LastTimeRise),
		LastTimeFall:     fromDbNullDateTime(time.RFC3339, r.LastTimeFall),
		LastTimeBullBear: fromDbNullDateTime(time.RFC3339, r.LastTimeBullBear),
	}

	return
}

// Only updates notify flag columns:
func (api *API) UpdateStock(n *Stock) (err error) {
	_, err = api.db.Exec(`
update Stock
set TStopPercent = ?2,
    BuyStopPrice = ?3,
    SellStopPrice = ?4,
    RisePercent = ?5,
    FallPercent = ?6,
    NotifyTStop = ?7,
    NotifyBuyStop = ?8,
    NotifySellStop = ?9,
    NotifyRise = ?10,
    NotifyFall = ?11,
    NotifyBullBear = ?12,
    BuyDate = ?13,
    BuyPrice = ?14,
    Shares = ?15
where StockID = ?1`,
		int64(n.StockID),
		toDbNullDecimal(n.TStopPercent, 2),
		toDbNullDecimal(n.BuyStopPrice, 2),
		toDbNullDecimal(n.SellStopPrice, 2),
		toDbNullDecimal(n.RisePercent, 2),
		toDbNullDecimal(n.FallPercent, 2),
		toDbBool(n.NotifyTStop),
		toDbBool(n.NotifyBuyStop),
		toDbBool(n.NotifySellStop),
		toDbBool(n.NotifyRise),
		toDbBool(n.NotifyFall),
		toDbBool(n.NotifyBullBear),
		toDbDateTime(n.BuyDate),
		toDbDecimal(n.BuyPrice, 2),
		n.Shares,
	)
	return
}

// Only updates last notification times:
func (api *API) UpdateNotifyTimes(n *Stock) (err error) {
	_, err = api.db.Exec(`
update Stock
set LastTimeTStop = ?2,
    LastTimeBuyStop = ?3,
	LastTimeSellStop = ?4,
	LastTimeRise = ?5,
	LastTimeFall = ?6,
	LastTimeBullBear = ?7
where StockID = ?1`,
		int64(n.StockID),
		toDbNullDateTime(time.RFC3339, n.LastTimeTStop),
		toDbNullDateTime(time.RFC3339, n.LastTimeBuyStop),
		toDbNullDateTime(time.RFC3339, n.LastTimeSellStop),
		toDbNullDateTime(time.RFC3339, n.LastTimeRise),
		toDbNullDateTime(time.RFC3339, n.LastTimeFall),
		toDbNullDateTime(time.RFC3339, n.LastTimeBullBear),
	)
	return
}

// Removes a stock:
func (api *API) RemoveStock(stockID StockID) (err error) {
	_, err = api.db.Exec(`delete from Stock where StockID = ?1`, int64(stockID))
	return
}

func projectDetails(rows []dbDetail) (details []StockDetail, err error) {
	// Copy raw DB rows into OwnedDetails records:
	details = make([]StockDetail, 0, len(rows))
	for _, r := range rows {
		s := &Stock{
			StockID:   StockID(r.StockID),
			UserID:    UserID(r.UserID),
			Symbol:    r.Symbol,
			BuyDate:   fromDbDateTime(dateFmt, r.BuyDate),
			BuyPrice:  fromDbDecimal(r.BuyPrice),
			Shares:    r.Shares,
			IsWatched: fromDbBool(r.IsWatched),

			TStopPercent:     fromDbNullDecimal(r.TStopPercent),
			BuyStopPrice:     fromDbNullDecimal(r.BuyStopPrice),
			SellStopPrice:    fromDbNullDecimal(r.SellStopPrice),
			RisePercent:      fromDbNullDecimal(r.RisePercent),
			FallPercent:      fromDbNullDecimal(r.FallPercent),
			NotifyTStop:      fromDbBool(r.NotifyTStop),
			NotifyBuyStop:    fromDbBool(r.NotifyBuyStop),
			NotifySellStop:   fromDbBool(r.NotifySellStop),
			NotifyRise:       fromDbBool(r.NotifyRise),
			NotifyFall:       fromDbBool(r.NotifyFall),
			NotifyBullBear:   fromDbBool(r.NotifyBullBear),
			LastTimeTStop:    fromDbNullDateTime(time.RFC3339, r.LastTimeTStop),
			LastTimeBuyStop:  fromDbNullDateTime(time.RFC3339, r.LastTimeBuyStop),
			LastTimeSellStop: fromDbNullDateTime(time.RFC3339, r.LastTimeSellStop),
			LastTimeRise:     fromDbNullDateTime(time.RFC3339, r.LastTimeRise),
			LastTimeFall:     fromDbNullDateTime(time.RFC3339, r.LastTimeFall),
			LastTimeBullBear: fromDbNullDateTime(time.RFC3339, r.LastTimeBullBear),
		}

		d := &Detail{
			CurrPrice:       fromDbNullDecimal(r.CurrPrice),
			CurrHour:        fromDbNullDateTime(time.RFC3339, r.CurrHour),
			FetchedDateTime: fromDbNullDateTime(time.RFC3339, r.FetchedDateTime),

			N1CloseDate:  fromDbNullDateTime(time.RFC3339, r.N1CloseDate),
			N1ClosePrice: fromDbNullDecimal(r.N1ClosePrice),
			N1SMAPercent: fromDbNullFloat64(r.N1SMAPercent),
			N1Avg200Day:  fromDbNullFloat64(r.N1Avg200Day),
			N1Avg50Day:   fromDbNullFloat64(r.N1Avg50Day),

			N2CloseDate:  fromDbNullDateTime(time.RFC3339, r.N2CloseDate),
			N2ClosePrice: fromDbNullDecimal(r.N2ClosePrice),
			N2SMAPercent: fromDbNullFloat64(r.N2SMAPercent),

			// TStopPrice
			// GainLossPercent
			// GainLossDollar
		}

		currPrice := fromDbNullDecimal(r.CurrPrice)
		buyPriceFlt := RatToFloat(s.BuyPrice.Value)

		if s.Shares >= 0 {
			// Owned (or watched):

			if s.TStopPercent.Valid && r.HighestClose.Valid {
				// ((100 - stopPercent) * 0.01) * highestClose
				d.TStopPrice = NullDecimal{Value: new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Sub(ToRat("100"), s.TStopPercent.Value), ToRat("0.01"))), FloatToRat(r.HighestClose.Float64)), Valid: true}
			}

			if currPrice.Valid {
				// gain% = ((currPrice / buyPrice) - 1) * 100
				d.GainLossPercent = NullFloat64{Value: (((RatToFloat(currPrice.Value) / buyPriceFlt) - 1.0) * 100.0), Valid: true}

				// gain$ = (currPrice - buyPrice) * shares
				d.GainLossDollar = NullDecimal{Value: new(big.Rat).Mul(new(big.Rat).Sub(currPrice.Value, s.BuyPrice.Value), IntToRat(s.Shares)), Valid: true}
			} else {
				d.GainLossPercent = NullFloat64{Valid: false}
				d.GainLossDollar = NullDecimal{Valid: false}
			}
		} else if s.Shares < 0 {
			// Shorted:

			if s.TStopPercent.Valid && r.LowestClose.Valid {
				// ((100 + stopPercent) * 0.01) * lowestClose
				d.TStopPrice = NullDecimal{Value: new(big.Rat).Mul((new(big.Rat).Mul(new(big.Rat).Add(ToRat("100"), s.TStopPercent.Value), ToRat("0.01"))), FloatToRat(r.LowestClose.Float64)), Valid: true}
			}

			if currPrice.Valid {
				// gain% = ((buyPrice / currPrice) - 1) * 100
				d.GainLossPercent = NullFloat64{Value: (((buyPriceFlt / RatToFloat(currPrice.Value)) - 1.0) * 100.0), Valid: true}

				// gain$ = (currPrice - buyPrice) * shares
				d.GainLossDollar = NullDecimal{Value: new(big.Rat).Mul(new(big.Rat).Sub(currPrice.Value, s.BuyPrice.Value), IntToRat(s.Shares)), Valid: true}
			} else {
				d.GainLossPercent = NullFloat64{Valid: false}
				d.GainLossDollar = NullDecimal{Valid: false}
			}
		}

		sd := StockDetail{
			Stock:  *s,
			Detail: *d,
		}

		// Add to list:
		details = append(details, sd)
	}

	return
}

func (api *API) GetStockDetailsForUser(userID UserID) (details []StockDetail, err error) {
	rows := make([]dbDetail, 0, 6)

	err = api.db.Select(&rows, `
select StockID, `+stockCols+`
     , CurrPrice, CurrHour, FetchedDateTime
     , N1CloseDate, N1ClosePrice, N1SMAPercent, N1Avg200Day, N1Avg50Day
     , N2CloseDate, N2ClosePrice, N2SMAPercent
     , LowestClose, HighestClose
from StockDetail s
where (s.UserID = ?1)
  and (datetime(s.CurrHour) = (select max(datetime(h.DateTime)) from StockHourly h where h.Symbol = s.Symbol))
order by s.Symbol ASC, s.BuyDate ASC, s.Shares ASC`, int64(userID))
	if err != nil {
		return
	}

	details, err = projectDetails(rows)
	return
}

func (api *API) GetStockDetailsForSymbol(symbol string) (details []StockDetail, err error) {
	rows := make([]dbDetail, 0, 6)

	err = api.db.Select(&rows, `
select StockID, `+stockCols+`
     , CurrPrice, CurrHour, FetchedDateTime
     , N1CloseDate, N1ClosePrice, N1SMAPercent, N1Avg200Day, N1Avg50Day
     , N2CloseDate, N2ClosePrice, N2SMAPercent
     , LowestClose, HighestClose
from StockDetail s
where (s.Symbol = ?1)
  and (datetime(s.CurrHour) = (select max(datetime(h.DateTime)) from StockHourly h where h.Symbol = s.Symbol))
order by s.Symbol ASC, s.BuyDate ASC, s.Shares ASC`, symbol)
	if err != nil {
		return
	}

	details, err = projectDetails(rows)
	return
}
