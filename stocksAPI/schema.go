package stocksAPI

import "time"

// sqlite related imports:
import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// Opens the DB and creates the table schema (if not exists):
func NewAPI(dbPath string) (api *API, err error) {
	// using sqlite 3.8.0 release
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		db.Close()
		return nil, err
	}

	// TODO: track schema version and add data migration code.

	api = &API{db: db}

	// Track historical stock data:
	api.ddl(`
create table if not exists StockHistory (
	Symbol TEXT NOT NULL,
	Date TEXT NOT NULL,
	TradeDayIndex INTEGER NOT NULL,
	Closing TEXT NOT NULL,
	Opening TEXT NOT NULL,
	Low TEXT NOT NULL,
	High TEXT NOT NULL,
	Volume INTEGER NOT NULL,
	CONSTRAINT PK_StockHistory PRIMARY KEY (Symbol, Date)
)`,
		// Index for historical data:
		`
create index if not exists IX_StockHistory_Closing on StockHistory (
	Symbol ASC,
	Date ASC,
	Closing ASC
)`,
		// StockStats to store trend information per stock per date:
		`
create table if not exists StockStats (
	Symbol TEXT NOT NULL,
	Date TEXT NOT NULL,
	TradeDayIndex INTEGER NOT NULL,
	Avg200Day TEXT NOT NULL,
	Avg50Day TEXT NOT NULL,
	SMAPercent TEXT NOT NULL,	-- simple moving average
	CONSTRAINT PK_StockStats PRIMARY KEY (Symbol, Date)
)`,
		// Index for trend data:
		`
create index if not exists IX_StockStats on StockStats (
	Symbol ASC,
	Date ASC,
	Avg200Day,
	Avg50Day,
	SMAPercent
)`,
		// Track hourly stock data:
		`
create table if not exists StockHourly (
	Symbol TEXT NOT NULL,
	DateTime TEXT NOT NULL,
	Current TEXT NOT NULL,
	CONSTRAINT PK_StockHourly PRIMARY KEY (Symbol, DateTime)
)`,
		// Index for hourly data:
		`
create index if not exists IX_StockHourly on StockHourly (
	Symbol ASC,
	DateTime ASC,
	Current
)`,
		// Create user tables:
		`
create table if not exists User (
	PrimaryEmail TEXT NOT NULL,
	Name TEXT NOT NULL,
	NotificationTimeout INTEGER,
	CONSTRAINT PK_User PRIMARY KEY (PrimaryEmail)
)`, `
create table if not exists UserEmail (
	Email TEXT NOT NULL,
	UserID INTEGER NOT NULL,
	CONSTRAINT PK_UserEmail PRIMARY KEY (Email)
)`,
		// Index for user emails:
		`
create unique index if not exists IX_UserEmail on UserEmail (
	Email ASC,
	UserID
)`,
		// Owned stocks:
		`
create table if not exists StockOwned (
	UserID INTEGER NOT NULL,
	Symbol TEXT NOT NULL,
	BuyDate TEXT NOT NULL,
	IsEnabled INTEGER NOT NULL,

	BuyPrice TEXT NOT NULL,
	Shares TEXT NOT NULL,
	TStopPercent TEXT NOT NULL,
	LastTStopNotifyTime TEXT
)`, `
create index if not exists IX_StockOwned on StockOwned (
	UserID ASC,
	Symbol ASC
)`,
		// Watched stocks:
		`
create table if not exists StockWatch (
	UserID INTEGER NOT NULL,
	Symbol TEXT NOT NULL,
	IsEnabled INTEGER NOT NULL,

	StartDate TEXT NOT NULL,
	StartPrice TEXT NOT NULL,
	TStopPercent TEXT NOT NULL,
	LastTStopNotifyTime TEXT,
	CONSTRAINT PK_StockWatch PRIMARY KEY (UserID, Symbol)
)`, `
create index if not exists IX_StockWatch on StockWatch (
	UserID ASC,
	Symbol ASC,
	IsEnabled
)`)

	// Create VIEWs:
	api.ddl(`
create view if not exists StockOwnedDetail
as
select o.rowid as ID, o.UserID, o.Symbol, o.BuyDate, o.IsEnabled, o.BuyPrice, o.Shares, o.TStopPercent, o.LastTStopNotifyTime
     , h.Current as CurrPrice, h.DateTime as CurrHour
     , t.Date, t.Avg200Day, t.Avg50Day, t.SMAPercent
     , e.HighestClose, e.LowestClose
from StockOwned o
join StockStats t on t.Symbol = h.Symbol and t.TradeDayIndex = (select max(TradeDayIndex) from StockHistory where Symbol = h.Symbol)
join StockHourly h on h.Symbol = o.Symbol
join (
	select o.rowid, h.Symbol, min(cast(h.Closing as real)) as LowestClose, max(cast(h.Closing as real)) as HighestClose
	from StockOwned o
	join StockHistory h on h.Symbol = o.Symbol
	where datetime(h.Date) >= datetime(o.BuyDate)
	group by o.rowid, h.Symbol
) e on e.rowid = o.rowid`)

	// Get today's date in NY time:
	api.today = TruncDate(time.Now().In(LocNY))

	// Find the last weekday trading date:
	// NOTE(jsd): Might screw up around DST changeover dates; who cares.
	api.lastTradingDate = api.today.Add(time.Hour * time.Duration(-24))
	for IsWeekend(api.lastTradingDate) {
		api.lastTradingDate = api.lastTradingDate.Add(time.Hour * time.Duration(-24))
	}

	// Success!
	return api, nil
}
