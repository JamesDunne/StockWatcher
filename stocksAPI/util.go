package stocksAPI

import (
	"fmt"
	"strings"
	"time"
)

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// ------------------------------- private API utility functions:

func (api *API) ddl(cmd string) (err error) {
	if _, err = api.db.Exec(cmd); err != nil {
		api.db.Close()
		return fmt.Errorf("%s\n%s", cmd, err)
	}
	return nil
}

// Gets a single scalar value from a DB query:
func (api *API) getScalar(query string, args ...interface{}) (value interface{}, err error) {
	// Call QueryRowx to get a raw Row result:
	row := api.db.QueryRowx(query, args...)
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
func (api *API) getScalars(query string, args ...interface{}) (slice []interface{}, err error) {
	// Call QueryRowx to get a raw Row result:
	row := api.db.QueryRowx(query, args...)
	if err = row.Err(); err != nil {
		return
	}

	// Get the column slice:
	slice, err = row.SliceScan()
	return
}

// Execute a database action in a transaction:
func (api *API) tx(action func(tx *sqlx.Tx) error) (err error) {
	tx, err := api.db.Beginx()
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
func (api *API) bulkInsert(tableName string, columns []string, rows [][]interface{}) (err error) {
	// Run in a transaction:
	return api.tx(func(tx *sqlx.Tx) (err error) {
		// Prepare insert statement:
		// e.g. `insert into StockHistory (Symbol, Date, Closing, Opening, High, Low, Volume) values (?1,?2,?3,?4,?5,?6,?7)`

		paramIdents := make([]string, 0, len(columns))
		for i := 0; i < len(columns); i++ {
			paramIdents = append(paramIdents, fmt.Sprintf("?%d", i+1))
		}

		stmtInsert, err := tx.Preparex(`insert or ignore into ` + tableName + ` (` + strings.Join(columns, ",") + `) values (` + strings.Join(paramIdents, ",") + `)`)
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

func parseNullTime(format string, v interface{}) *time.Time {
	if v == nil {
		return nil
	}

	t, err := time.Parse(format, v.(string))
	if err != nil {
		fmt.Println(err)
		return nil
	}
	return &t
}

func minNullTime(a, b *time.Time) *time.Time {
	if a == nil && b == nil {
		return nil
	} else if a == nil {
		return b
	} else if b == nil {
		return a
	} else {
		if (*a).After(*b) {
			return b
		} else {
			return a
		}
	}
}
