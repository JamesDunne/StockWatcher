package stocksdb

// sqlite related imports:
import "github.com/jmoiron/sqlx"

// Creates the DB schema for stocks:
func CreateSchema(path string) (db *sqlx.DB, err error) {
	// using sqlite 3.8.0 release
	db, err = sqlx.Connect("sqlite3", path)
	if err != nil {
		db.Close()
		return
	}

	// Track historical stock data:
	if _, err = db.Exec(`
create table if not exists StockHistory (
	Symbol TEXT NOT NULL,
	Date TEXT NOT NULL,
	Closing TEXT NOT NULL,
	Opening TEXT NOT NULL,
	Low TEXT NOT NULL,
	High TEXT NOT NULL,
	Volume INTEGER NOT NULL,
	CONSTRAINT PK_StockHistory PRIMARY KEY (Symbol, Date)
)`); err != nil {
		db.Close()
		return
	}

	// Index for historical data:
	if _, err = db.Exec(`
create index if not exists IX_StockHistory_Closing on StockHistory (
	Symbol ASC,
	Date ASC,
	Closing ASC
)`); err != nil {
		db.Close()
		return
	}

	// Create user table:
	// TODO(jsd): OpenID sessions support
	if _, err = db.Exec(`
create table if not exists User (
	Email TEXT NOT NULL,
	Name TEXT NOT NULL,
	NotificationTimeout INTEGER,	-- in seconds
	CONSTRAINT PK_User PRIMARY KEY (Email)
)`); err != nil {
		db.Close()
		return
	}

	// Index for users:
	if _, err = db.Exec(`
create index if not exists IX_User on User (
	Email ASC,
	Name ASC
)`); err != nil {
		db.Close()
		return
	}

	// Track user-owned stocks and calculate a trailing stop price:
	if _, err = db.Exec(`
create table if not exists StockOwned (
	UserID INTEGER NOT NULL,
	Symbol TEXT NOT NULL,
	IsStopEnabled INTEGER NOT NULL,
	PurchaseDate TEXT NOT NULL,
	PurchasePrice TEXT NOT NULL,
	StopPercent TEXT NOT NULL,
	StopLastNotificationDate TEXT,
	CONSTRAINT PK_StockOwned PRIMARY KEY (UserID, Symbol)
)`); err != nil {
		db.Close()
		return
	}

	if _, err = db.Exec(`
create index if not exists IX_StockOwned on StockOwned (
	UserID ASC,
	Symbol ASC,
	IsStopEnabled,
	PurchaseDate,
	PurchasePrice
)`); err != nil {
		db.Close()
		return
	}

	return
}
