package stocks

import (
	"fmt"
	"math/big"
	"strings"
	"time"
)

import (
	"database/sql"
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

func toDbBool(v bool) int64 {
	if v {
		return int64(1)
	} else {
		return int64(0)
	}
}

func toDbNullDecimal(v NullDecimal, prec int) sql.NullString {
	if !v.Valid {
		return sql.NullString{String: "", Valid: false}
	} else {
		return sql.NullString{String: v.Value.FloatString(prec), Valid: true}
	}
}

func toDbDecimal(v Decimal, prec int) string {
	if v.Value == nil {
		panic(fmt.Errorf("Unexpected nil value of *big.Rat for toDbDecimal!"))
	} else {
		return v.Value.FloatString(prec)
	}
}

func toDbDateTime(v DateTime) string {
	return v.Value.Format(time.RFC3339)
}

func toDbNullDateTime(format string, v NullDateTime) sql.NullString {
	if !v.Valid {
		return sql.NullString{String: "", Valid: false}
	}

	return sql.NullString{String: v.Value.Format(format), Valid: true}
}

func fromDbNullDecimal(v sql.NullString) NullDecimal {
	if !v.Valid {
		return NullDecimal{Value: nil, Valid: false}
	}
	d := NullDecimal{
		Value: new(big.Rat),
		Valid: true,
	}
	d.Value.SetString(v.String)
	return d
}

func fromDbDecimal(v string) Decimal {
	if v == "" {
		panic("Unexpected empty string value in fromDbDecimal!")
	}
	d := new(big.Rat)
	d.SetString(v)
	return Decimal{Value: d}
}

func fromDbNullFloat64(v sql.NullFloat64) NullFloat64 {
	if !v.Valid {
		return NullFloat64{Valid: false}
	}

	return NullFloat64{Value: v.Float64, Valid: true}
}

func fromDbBool(i int64) bool {
	if i == 0 {
		return false
	} else {
		return true
	}
}

func fromDbNullDateTime(format string, v sql.NullString) NullDateTime {
	if !v.Valid {
		return NullDateTime{Valid: false}
	}

	t, err := time.Parse(format, v.String)
	if err != nil {
		panic(err)
	}
	return NullDateTime{Value: t, Valid: true}
}

// parses the date/time, assuming NY timezone:
func fromDbDateTime(format string, str string) DateTime {
	t, err := time.ParseInLocation(time.RFC3339, str, LocNY)
	if err != nil {
		panic(err)
	}
	return DateTime{Value: t}
}

// remove the time component of a datetime to get just a date at 00:00:00
func TruncDate(t time.Time) time.Time {
	hour, min, sec := t.Clock()
	nano := t.Nanosecond()

	d := time.Duration(0) - (time.Duration(nano) + time.Duration(sec)*time.Second + time.Duration(min)*time.Minute + time.Duration(hour)*time.Hour)
	return t.Add(d)
}

func minNullTime(a, b NullDateTime) NullDateTime {
	if !a.Valid && !b.Valid {
		return NullDateTime{Valid: false}
	} else if !a.Valid {
		return b
	} else if !b.Valid {
		return a
	} else {
		if (a.Value).After(b.Value) {
			return b
		} else {
			return a
		}
	}
}

func commaDelimDblQuote(strs ...string) string {
	str := ""
	for i, s := range strs {
		if i > 0 {
			str += ","
		}
		str += fmt.Sprintf(`"%s"`, s)
	}
	return str
}

func commaDelimSngQuote(strs ...string) string {
	str := ""
	for i, s := range strs {
		if i > 0 {
			str += ","
		}
		str += fmt.Sprintf(`'%s'`, s)
	}
	return str
}

func parametersList(start, count int) string {
	str := ""
	for i := 0; i < count; i++ {
		if i > 0 {
			str += ","
		}
		str += fmt.Sprintf("?%d", (i + start))
	}
	return str
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

// Check if the date is on a weekend:
func IsWeekend(date time.Time) bool {
	return date.Weekday() == 0 || date.Weekday() == 6
}
