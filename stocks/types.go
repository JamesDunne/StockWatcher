package stocks

import (
	"fmt"
	"math/big"
	"time"
)

type Decimal struct {
	Value *big.Rat
}

func (d Decimal) String() string {
	return d.Value.FloatString(2)
}

func (d Decimal) CurrencyString() string {
	if d.Value.Sign() > 0 {
		return "+" + d.Value.FloatString(2)
	} else if d.Value.Sign() == 0 {
		return " " + d.Value.FloatString(2)
	} else {
		// Includes - sign:
		return d.Value.FloatString(2)
	}
}

type NullDecimal struct {
	Value *big.Rat
	Valid bool
}

func (d NullDecimal) String() string {
	if d.Valid {
		return d.Value.FloatString(2)
	} else {
		return ""
	}
}

func (d NullDecimal) CurrencyString() string {
	if !d.Valid {
		return ""
	}
	if d.Value.Sign() > 0 {
		return "+" + d.Value.FloatString(2)
	} else if d.Value.Sign() == 0 {
		return " " + d.Value.FloatString(2)
	} else {
		// Includes - sign:
		return d.Value.FloatString(2)
	}
}

var DecimalNull = NullDecimal{Value: nil, Valid: false}

func ToDecimal(v string) Decimal {
	r := new(big.Rat)
	r.SetString(v)
	return Decimal{Value: r}
}

func ToNullDecimal(v string) NullDecimal {
	if v == "" {
		return NullDecimal{Valid: false}
	}
	r := new(big.Rat)
	r.SetString(v)
	return NullDecimal{Value: r, Valid: true}
}

// --------------

type Float64 struct {
	Value float64
}

func (d Float64) String() string {
	return fmt.Sprintf("%.2f", d.Value)
}

type NullFloat64 struct {
	Value float64
	Valid bool
}

func (d NullFloat64) String() string {
	if d.Valid {
		return fmt.Sprintf("%.2f", d.Value)
	} else {
		return ""
	}
}

type DateTime struct {
	Value time.Time
}

func (d DateTime) Format(format string) string {
	return d.Value.Format(format)
}

func (d DateTime) DateString() string {
	return d.Value.Format(dateFmt)
}

type NullDateTime struct {
	Value time.Time
	Valid bool
}

func (d NullDateTime) Format(format string) string {
	if !d.Valid {
		return ""
	}
	return d.Value.Format(format)
}

var DateTimeNull = NullDateTime{Valid: false}

func ToDateTime(format, s string) DateTime {
	t, err := time.Parse(format, s)
	if err != nil {
		panic(err)
	}
	return DateTime{Value: t}
}

func ToNullDateTime(format, s string) NullDateTime {
	if s == "" {
		return NullDateTime{Valid: false}
	}
	t, err := time.Parse(format, s)
	if err != nil {
		panic(err)
	}
	return NullDateTime{Value: t, Valid: true}
}
