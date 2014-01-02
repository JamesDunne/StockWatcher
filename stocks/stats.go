package stocks

// general stuff:
import (
	"time"
)

// Our own packages:
import (
	"database/sql"
	"github.com/JamesDunne/StockWatcher/yql"
	"github.com/jmoiron/sqlx"
)

// Get the earliest buy date for a symbol.
func (api *API) GetMinBuyDate(symbol string) NullDateTime {
	// Find earliest date of interest for history:
	row := struct {
		Min sql.NullString `db:"Min"`
	}{}
	err := api.db.Get(&row, `select min(datetime(BuyDate)) as Min from Stock where Symbol = ?1`, symbol)
	if err != nil {
		panic(err)
	}

	return fromDbNullDateTime(sqliteFmt, row.Min)
}

// Deletes all historical and statistical data for a symbol.
func (api *API) DeleteHistory(symbol string) {
	err := api.tx(func(tx *sqlx.Tx) (err error) {
		_, err = api.db.Exec(`delete from StockHistory where Symbol = ?1`, symbol)
		if err != nil {
			return err
		}
		_, err = api.db.Exec(`delete from StockStats where Symbol = ?1`, symbol)
		if err != nil {
			return err
		}
		return
	})
	if err != nil {
		panic(err)
	}
}

// Fetches historical data from Yahoo Finance into the database.
func (api *API) RecordHistory(symbol string) {
	var startDate time.Time

	// Fetch earliest date of interest for symbol:
	lastDateTime, lastTradeDay, err := api.GetLastTradeDay(symbol)
	if err != nil {
		minDate := api.GetMinBuyDate(symbol)
		if !minDate.Valid {
			startDate = api.lastTradingDate
		} else {
			startDate = minDate.Value
		}

		// Take it back at least 42 weeks to get the 200-day moving average:
		startDate = startDate.Add(time.Duration(-42*7*24) * time.Hour)
		lastTradeDay = 0
	} else {
		startDate = lastDateTime.Value
	}

	// Do we need to fetch history?
	if !startDate.Before(api.lastTradingDate) {
		return
	}

	// Fetch the historical data:
	hist, err := yql.GetHistory(symbol, startDate, api.lastTradingDate)
	if err != nil {
		panic(err)
	}

	// Bulk insert the historical data into the StockHistory table:
	rows := make([][]interface{}, 0, len(hist))
	for i, h := range hist {
		// Store dates as RFC3339 in the NYC timezone:
		date, err := time.ParseInLocation(dateFmt, h.Date, LocNY)
		if err != nil {
			panic(err)
		}

		// Only record dates after last-fetched dates:
		if date.After(time.Time(startDate)) {
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
			panic(err)
		}
	}

	// Calculates per-day trends and records them to the database.
	_, err = api.db.Exec(`
replace into StockStats (Symbol, Date, TradeDayIndex, Avg200Day, Avg50Day, SMAPercent)
select Symbol, Date, TradeDayIndex, Avg200, Avg50, ((Avg50 / Avg200) - 1) * 100 as SMAPercent
from (
	select h.Symbol, h.Date, h.TradeDayIndex
	     , (select avg(cast(Closing as real)) from StockHistory h0 where (h0.Symbol = h.Symbol) and (h0.TradeDayIndex >= (h.TradeDayIndex - 200))) as Avg200
	     , (select avg(cast(Closing as real)) from StockHistory h0 where (h0.Symbol = h.Symbol) and (h0.TradeDayIndex >= (h.TradeDayIndex - 50))) as Avg50
	from StockHistory h
	where (h.Symbol = ?1)
	  and (h.TradeDayIndex > 200)
)`, symbol)
	if err != nil {
		panic(err)
	}
	return
}

// Gets the current time truncated down 15 minutes:
func truncTime(t time.Time) time.Time   { return t.Truncate(time.Minute * time.Duration(15)) }
func (api *API) CurrentHour() time.Time { return truncTime(time.Now()) }

// Checks if the current hourly price has been fetched from Yahoo or not and fetches it into the StockHourly table if needed.
func (api *API) GetCurrentHourlyPrices(force bool, symbols ...string) (prices map[string]Decimal) {
	currHour := api.CurrentHour()

	toFetch := make([]string, 0, len(symbols))
	prices = make(map[string]Decimal)
	if force {
		// Forcefully fetch all symbols from Yahoo Finance:
		for _, symbol := range symbols {
			toFetch = append(toFetch, symbol)
		}
	} else {
		for _, symbol := range symbols {
			row := struct {
				Max sql.NullString `db:"Max"`
			}{}
			err := api.db.Get(&row, `select max(datetime(DateTime)) as Max from StockHourly where Symbol = ?1`, symbol)
			if err != nil {
				panic(err)
			}
			lastTime := fromDbNullDateTime(sqliteFmt, row.Max)

			// Determine if we need to fetch from Yahoo or not:
			needFetch := false
			if !lastTime.Valid {
				needFetch = true
			} else {
				lastHour := truncTime(time.Time(lastTime.Value))
				if lastHour.Before(currHour) {
					needFetch = true
				}
			}

			// TODO(jsd): could break this out to separate single query with IN clause
			if !needFetch {
				row := struct {
					Current sql.NullString `db:"Current"`
				}{}
				err := api.db.Get(&row, `select Current from StockHourly where Symbol = ?1 and DateTime = ?2`, symbol, currHour.Format(time.RFC3339))
				if err != nil {
					panic(err)
				}

				if row.Current.Valid {
					prices[symbol] = ToDecimal(row.Current.String)
					continue
				}
			}

			// Add it to the list of symbols to be fetched from Yahoo:
			toFetch = append(toFetch, symbol)
		}
	}

	// Get current prices from Yahoo:
	if len(toFetch) > 0 {
		quotes, err := yql.GetQuotes(toFetch...)
		if err != nil {
			panic(err)
		}

		for _, quote := range quotes {
			// Record the current hourly price:
			_, err = api.db.Exec(`replace into StockHourly (Symbol, DateTime, Current, FetchedDateTime) values (?1, ?2, ?3, ?4)`,
				quote.Symbol,
				currHour.Format(time.RFC3339),
				quote.Price.FloatString(2),
				time.Now().In(LocNY).Format(time.RFC3339),
			)
			if err != nil {
				panic(err)
			}

			// Fill in the price map:
			prices[quote.Symbol] = Decimal{Value: quote.Price}
		}
	}

	return
}
