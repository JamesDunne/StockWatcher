// database
package main

import "fmt"
import "strings"

// sqlite related imports:
import _ "github.com/mattn/go-sqlite3"
import "github.com/jmoiron/sqlx"

// Gets a single scalar value from a DB query:
func dbGetScalar(db *sqlx.DB, query string, args ...interface{}) (value interface{}, err error) {
	// Call QueryRowx to get a raw Row result:
	row := db.QueryRowx(query, args...)
	if err = row.Err(); err != nil {
		return
	}

	// Get the column slice:
	slice, err := row.SliceScan()
	if err != nil {
		return nil, err
	}

	if len(slice) == 0 {
		return nil, nil
	}

	return slice[0], nil
}

// Gets a slice of scalar values from a DB query:
func dbGetScalars(db *sqlx.DB, query string, args ...interface{}) (slice []interface{}, err error) {
	// Call QueryRowx to get a raw Row result:
	row := db.QueryRowx(query, args...)
	if err = row.Err(); err != nil {
		return
	}

	// Get the column slice:
	slice, err = row.SliceScan()
	return
}

// Execute a database action in a transaction:
func dbTx(db *sqlx.DB, action func(tx *sqlx.Tx) error) (err error) {
	tx, err := db.Beginx()
	if err != nil {
		return
	}

	err = action(tx)
	if err != nil {
		//tx.Tx.Abort()
		return
	}

	// Commit the transaction:
	err = tx.Tx.Commit()
	return
}

// Does a bulk insert of data into a single table using a transaction to make it quick:
func dbBulkInsert(db *sqlx.DB, tableName string, columns []string, rows [][]interface{}) (err error) {
	// Run in a transaction:
	return dbTx(db, func(tx *sqlx.Tx) (err error) {
		// Prepare insert statement:
		// e.g. `insert into StockHistory (Symbol, Date, Closing, Opening, High, Low, Volume) values (?1,?2,?3,?4,?5,?6,?7)`

		paramIdents := make([]string, 0, len(columns))
		for i := 0; i < len(columns); i++ {
			paramIdents = append(paramIdents, fmt.Sprintf("?%d", i+1))
		}

		stmtInsert, err := tx.Preparex(`insert into ` + tableName + ` (` + strings.Join(columns, ",") + `) values (` + strings.Join(paramIdents, ",") + `)`)
		if err != nil {
			return
		}

		// Insert each row:
		for _, row := range rows {
			stmtInsert.Execl(row...)
		}
		return
	})
}

// Creates the DB schema and inserts testing data:
func dbCreateSchema(path string) (db *sqlx.DB, err error) {
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

	// Add some test data:
	dbTx(db, func(tx *sqlx.Tx) (err error) {
		db.Execl(`insert or ignore into User (Email, Name, NotificationTimeout) values ('example@example.org', 'Example User', 15)`)
		db.Execl(`insert or ignore into StockOwned (UserID, Symbol, IsStopEnabled, PurchaseDate, PurchasePrice, StopPercent) values (1, 'MSFT', 1, '2012-09-01', '30.00', '0.1');`)
		return nil
	})

	return
}
