package stocksAPI

// general stuff:
import (
	//"math/big"
	"time"
)

// sqlite related imports:
import (
	//"database/sql"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Our own packages:
import (
//"github.com/JamesDunne/StockWatcher/yql"
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
type OwnedID int64
type WatchedID int64

// ------------------------- API functions:

const dateFmt = "2006-01-02"
const sqliteFmt = "2006-01-02 15:04:05"

// Releases all API resources:
func (api *API) Close() {
	api.db.Close()
	api.db = nil
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
	select Symbol from StockWatched where IsEnabled = 1
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

// Gets the last date trading occurred for a stock symbol.
func (api *API) GetLastTradeDay(symbol string) (date time.Time, tradeDay int64, err error) {
	row := struct {
		Date          string `db:"Date"`
		TradeDayIndex int64  `db:"TradeDayIndex"`
	}{}

	err = api.db.Get(&row, `select h.Date, h.TradeDayIndex from StockHistory h where (h.Symbol = ?1) and (h.TradeDayIndex = (select max(TradeDayIndex) from StockHistory where Symbol = h.Symbol))`, symbol)
	if err != nil {
		return
	}

	date = TruncDate(TradeDateTime(row.Date))
	tradeDay = row.TradeDayIndex
	err = nil
	return
}
