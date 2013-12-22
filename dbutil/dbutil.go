package dbutil

import "fmt"
import "strings"

import "github.com/jmoiron/sqlx"

// Gets a single scalar value from a DB query:
func GetScalar(db *sqlx.DB, query string, args ...interface{}) (value interface{}, err error) {
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
func GetScalars(db *sqlx.DB, query string, args ...interface{}) (slice []interface{}, err error) {
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
func Tx(db *sqlx.DB, action func(tx *sqlx.Tx) error) (err error) {
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
func BulkInsert(db *sqlx.DB, tableName string, columns []string, rows [][]interface{}) (err error) {
	// Run in a transaction:
	return Tx(db, func(tx *sqlx.Tx) (err error) {
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
