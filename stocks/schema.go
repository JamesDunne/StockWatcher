package stocks

import "time"

// sqlite related imports:
import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

const stockCols = "UserID,Symbol,BuyDate,BuyPrice,Shares,IsWatched,TStopPercent,BuyStopPrice,SellStopPrice,RisePercent,FallPercent,NotifyTStop,NotifyBuyStop,NotifySellStop,NotifyRise,NotifyFall,NotifyBullBear,LastTimeTStop,LastTimeBuyStop,LastTimeSellStop,LastTimeRise,LastTimeFall,LastTimeBullBear"
const stockColsS = "s.UserID,s.Symbol,s.BuyDate,s.BuyPrice,s.Shares,s.IsWatched,s.TStopPercent,s.BuyStopPrice,s.SellStopPrice,s.RisePercent,s.FallPercent,s.NotifyTStop,s.NotifyBuyStop,s.NotifySellStop,s.NotifyRise,s.NotifyFall,s.NotifyBullBear,s.LastTimeTStop,s.LastTimeBuyStop,s.LastTimeSellStop,s.LastTimeRise,s.LastTimeFall,s.LastTimeBullBear"

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
create index if not exists IX_StockHistory on StockHistory (
	Symbol ASC,
	TradeDayIndex ASC
)`,
		// StockStats to store stats per stock per date:
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
		// Index for stats:
		`
create index if not exists IX_StockStats on StockStats (
	Symbol ASC,
	TradeDayIndex ASC
)`,
		// Track hourly stock price:
		`
create table if not exists StockHourly (
	Symbol TEXT NOT NULL,
	DateTime TEXT NOT NULL,
	Current TEXT NOT NULL,
	FetchedDateTime TEXT NOT NULL,
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
	UserID INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
	Name TEXT NOT NULL,
	NotificationTimeout INTEGER NOT NULL
)`, `
create table if not exists UserEmail (
	Email TEXT NOT NULL,
	UserID INTEGER NOT NULL,
	IsPrimary INTEGER NOT NULL,
	CONSTRAINT PK_UserEmail PRIMARY KEY (Email, UserID)
)`,
		// Index for user emails:
		`
create unique index if not exists IX_UserEmail on UserEmail (
	Email ASC,
	UserID,
	IsPrimary
)`,
		// Per-user tracked stocks:
		`
create table if not exists Stock (
	StockID INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,

	UserID INTEGER NOT NULL,
	Symbol TEXT NOT NULL,
	BuyDate TEXT NOT NULL,
	BuyPrice TEXT NOT NULL,
	Shares INTEGER NOT NULL,
	IsWatched INTEGER NOT NULL,  -- 0 for owned, 1 for watched

	-- Notifications:
	TStopPercent TEXT,
	BuyStopPrice TEXT,
	SellStopPrice TEXT,
	RisePercent TEXT,
	FallPercent TEXT,
	NotifyTStop INTEGER NOT NULL,
	NotifyBuyStop INTEGER NOT NULL,
	NotifySellStop INTEGER NOT NULL,
	NotifyRise INTEGER NOT NULL,
	NotifyFall INTEGER NOT NULL,
	NotifyBullBear INTEGER NOT NULL,
	LastTimeTStop TEXT,
	LastTimeBuyStop TEXT,
	LastTimeSellStop TEXT,
	LastTimeRise TEXT,
	LastTimeFall TEXT,
	LastTimeBullBear TEXT
)`, `
create index if not exists IX_Stock on Stock (
	UserID ASC,
	Symbol ASC
)`)

	// Create VIEWs:
	api.ddl(
		// StockHistoryStats
		`drop view if exists StockHistoryStats`,
		`
create view if not exists StockHistoryStats
as
select h.Symbol, h.Date as CloseDate, h.TradeDayIndex, h.Closing as ClosePrice
     , s.Avg200Day, s.Avg50Day, s.SMAPercent
from StockHistory h
join StockStats s on s.Symbol = h.Symbol and s.TradeDayIndex = h.TradeDayIndex`,
		// StockDetail
		`drop view if exists StockDetail`,
		`
create view if not exists StockDetail
as
select s.StockID, `+stockColsS+`
     , h.Current as CurrPrice, h.DateTime as CurrHour
     , n1.CloseDate as N1CloseDate, n1.ClosePrice as N1ClosePrice, n1.SMAPercent as N1SMAPercent, n1.Avg200Day as N1Avg200Day, n1.Avg50Day as N1Avg50Day
     , n2.CloseDate as N2CloseDate, n2.ClosePrice as N2ClosePrice, n2.SMAPercent as N2SMAPercent
     , e.LowestClose, e.HighestClose
from Stock s
left join StockHourly h on h.Symbol = s.Symbol
left join StockHistoryStats n1 on n1.Symbol = h.Symbol and n1.TradeDayIndex = (select max(TradeDayIndex)-0 from StockHistory where Symbol = h.Symbol)
left join StockHistoryStats n2 on n2.Symbol = h.Symbol and n2.TradeDayIndex = (select max(TradeDayIndex)-1 from StockHistory where Symbol = h.Symbol)
left join (
	-- Find lowest and highest closing price since buy date per symbol:
	select s.StockID, h.Symbol
	     , min(cast(h.Closing as real)) as LowestClose
		 , max(cast(h.Closing as real)) as HighestClose
	from Stock s
	join StockHistory h on h.Symbol = s.Symbol
	where datetime(h.Date) >= datetime(s.BuyDate)
	group by s.StockID, h.Symbol
) e on e.StockID = s.StockID
order by s.Symbol ASC, s.BuyDate ASC`)

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
