package stocksAPI

// general stuff:
import (
	"math/big"
	"time"
)

// Our own packages:
import (
	"github.com/JamesDunne/StockWatcher/yql"
	"github.com/jmoiron/sqlx"
)

// Fetches historical data from Yahoo Finance into the database.
func (api *API) RecordHistory(symbol string) (err error) {
	// Fetch earliest date of interest for symbol:
	lastDate, lastTradeDay, err := api.GetLastTradeDay(symbol)
	if err != nil {
		// Find earliest date of interest for history:
		minBuyDateV, err := api.getScalar(`select min(datetime(BuyDate)) from StockOwned where Symbol = ?1`, symbol)
		if err != nil {
			return err
		}
		minStartDateV, err := api.getScalar(`select min(datetime(StartDate)) from StockWatched where Symbol = ?1`, symbol)
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
func (api *API) RecordStats(symbol string) (err error) {
	err = api.tx(func(tx *sqlx.Tx) (err error) {
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
		return
	})
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
