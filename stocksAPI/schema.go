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
	if err = api.ddl(`
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
)`); err != nil {
		return nil, err
	}

	// Index for historical data:
	if err = api.ddl(`
create index if not exists IX_StockHistory_Closing on StockHistory (
	Symbol ASC,
	Date ASC,
	Closing ASC
)`); err != nil {
		return nil, err
	}

	// StockHistoryTrend to store trend information per stock per date:
	if err = api.ddl(`
create table if not exists StockHistoryTrend (
	Symbol TEXT NOT NULL,
	Date TEXT NOT NULL,
	Avg200Day TEXT NOT NULL,
	Avg50Day TEXT NOT NULL,
	SMAPercent TEXT NOT NULL,	-- simple moving average
	CONSTRAINT PK_StockHistoryTrend PRIMARY KEY (Symbol, Date)
)`); err != nil {
		return nil, err
	}

	// Index for trend data:
	if err = api.ddl(`
create index if not exists IX_StockHistoryTrend on StockHistoryTrend (
	Symbol ASC,
	Date ASC,
	Avg200Day,
	Avg50Day,
	SMAPercent
)`); err != nil {
		return nil, err
	}

	// Create user tables:
	if err = api.ddl(`
create table if not exists User (
	PrimaryEmail TEXT NOT NULL,
	Name TEXT NOT NULL,
	NotificationTimeout INTEGER,
	CONSTRAINT PK_User PRIMARY KEY (PrimaryEmail)
)`); err != nil {
		return nil, err
	}

	if err = api.ddl(`
create table if not exists UserEmail (
	Email TEXT NOT NULL,
	UserID INTEGER NOT NULL,
	CONSTRAINT PK_UserEmail PRIMARY KEY (Email)
)`); err != nil {
		return nil, err
	}

	// Index for user emails:
	if err = api.ddl(`
create unique index if not exists IX_UserEmail on UserEmail (
	Email ASC,
	UserID
)`); err != nil {
		return nil, err
	}

	// Owned stocks:
	if err = api.ddl(`
create table if not exists StockOwned (
	UserID INTEGER NOT NULL,
	Symbol TEXT NOT NULL,
	BuyDate TEXT NOT NULL,
	IsEnabled INTEGER NOT NULL,

	BuyPrice TEXT NOT NULL,
	Shares TEXT NOT NULL,
	StopPercent TEXT NOT NULL,
	LastNotificationDate TEXT,
	CONSTRAINT PK_StockOwned PRIMARY KEY (UserID, Symbol, BuyDate)
)`); err != nil {
		return nil, err
	}

	// Watched stocks:
	if err = api.ddl(`
create table if not exists StockWatch (
	UserID INTEGER NOT NULL,
	Symbol TEXT NOT NULL,
	IsEnabled INTEGER NOT NULL,

	StartDate TEXT NOT NULL,
	StartPrice TEXT NOT NULL,
	StopPercent TEXT NOT NULL,
	LastNotificationDate TEXT,
	CONSTRAINT PK_StockWatch PRIMARY KEY (UserID, Symbol)
)`); err != nil {
		return nil, err
	}

	if err = api.ddl(`
create index if not exists IX_StockWatch on StockWatch (
	UserID ASC,
	Symbol ASC,
	IsEnabled
)`); err != nil {
		return nil, err
	}

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
