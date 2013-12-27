package stocksAPI

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

import (
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

// ------------------------------- private API utility functions:

func (api *API) ddl(cmds ...string) {
	for _, cmd := range cmds {
		if _, err := api.db.Exec(cmd); err != nil {
			api.db.Close()
			panic(fmt.Errorf("%s\n%s", cmd, err))
		}
	}
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

// Converts a string into a `*big.Rat` which is an arbitrary precision rational number stored in decimal format
func ToRat(v string) *big.Rat {
	rat := new(big.Rat)
	rat.SetString(v)
	return rat
}

func IntToRat(v int64) *big.Rat {
	rat := new(big.Rat)
	rat.SetInt64(v)
	return rat
}

func FloatToRat(v float64) *big.Rat {
	rat := new(big.Rat)
	rat.SetFloat64(v)
	return rat
}

func RatToFloat(v *big.Rat) float64 {
	f, _ := v.Float64()
	return f
}

// remove the time component of a datetime to get just a date at 00:00:00
func TruncDate(t time.Time) time.Time {
	hour, min, sec := t.Clock()
	nano := t.Nanosecond()

	d := time.Duration(0) - (time.Duration(nano) + time.Duration(sec)*time.Second + time.Duration(min)*time.Minute + time.Duration(hour)*time.Hour)
	return t.Add(d)
}

// parses the date/time in RFC3339 format, assuming NY timezone:
func TradeDateTime(str string) time.Time {
	t, err := time.ParseInLocation(time.RFC3339, str, LocNY)
	if err != nil {
		panic(err)
	}
	return t
}

// parses the date in yyyy-MM-dd format, assuming NY timezone:
func TradeDate(str string) time.Time {
	t, err := time.ParseInLocation(dateFmt, str, LocNY)
	if err != nil {
		panic(err)
	}
	return t
}

func TradeSqliteDateTime(str string) time.Time {
	t, err := time.ParseInLocation(sqliteFmt, str, LocNY)
	if err != nil {
		panic(err)
	}
	return t
}

// Check if the date is on a weekend:
func IsWeekend(date time.Time) bool {
	return date.Weekday() == 0 || date.Weekday() == 6
}

func ToBool(i int) bool {
	if i == 0 {
		return false
	} else {
		return true
	}
}
