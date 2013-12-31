package stocks

import (
	"encoding/json"
	"fmt"
	"math/big"
	"strconv"
	"time"
)

type Decimal struct {
	Value *big.Rat
	// TODO: store precision here too.
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

func (d Decimal) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Value.FloatString(2))
}

func (d *Decimal) UnmarshalJSON(data []byte) error {
	str := new(string)
	err := json.Unmarshal(data, str)
	if err != nil {
		return err
	}
	d.Value = ToDecimal(*str).Value
	return nil
}

func ToDecimal(v string) Decimal {
	r := new(big.Rat)
	r.SetString(v)
	return Decimal{Value: r}
}

// --------------

type NullDecimal struct {
	Value *big.Rat
	// TODO: store precision here too.
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

func (d NullDecimal) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(d.Value.FloatString(2))
}

func (d *NullDecimal) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	if str == "" {
		d.Valid = false
		return nil
	}
	d.Value = ToDecimal(str).Value
	d.Valid = true
	return nil
}

var DecimalNull = NullDecimal{Value: nil, Valid: false}

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
	// TODO: store precision here too.
}

func (d Float64) String() string {
	return fmt.Sprintf("%.2f", d.Value)
}

func (d Float64) MarshalJSON() ([]byte, error) {
	return json.Marshal(fmt.Sprintf("%f", d.Value))
}

func (d *Float64) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return err
	}
	d.Value = f
	return nil
}

// --------------

type NullFloat64 struct {
	Value float64
	Valid bool
	// TODO: store precision here too.
}

func (d NullFloat64) String() string {
	if d.Valid {
		return fmt.Sprintf("%.2f", d.Value)
	} else {
		return ""
	}
}

func (d NullFloat64) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(fmt.Sprintf("%f", d.Value))
}

func (d *NullFloat64) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	if str == "" {
		d.Valid = false
		return nil
	}
	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return err
	}
	d.Value = f
	d.Valid = true
	return nil
}

func ToNullFloat64(v string) NullFloat64 {
	f, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return NullFloat64{Valid: false}
	}
	return NullFloat64{Value: f, Valid: true}
}

// --------------

type DateTime struct {
	Value time.Time
	// TODO: store format here too.
}

func (d DateTime) String() string {
	return d.Value.Format(time.RFC3339)
}

func (d DateTime) Format(format string) string {
	return d.Value.Format(format)
}

func (d DateTime) DateString() string {
	return d.Value.Format(dateFmt)
}

func (d DateTime) MarshalJSON() ([]byte, error) {
	return json.Marshal(d.Value.Format(time.RFC3339))
}

func (d *DateTime) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	t, err := time.ParseInLocation(time.RFC3339, str, LocNY)
	if err != nil {
		return err
	}
	d.Value = t
	return nil
}

// --------------

type NullDateTime struct {
	Value time.Time
	// TODO: store format here too.
	Valid bool
}

func (d NullDateTime) String() string {
	if !d.Valid {
		return ""
	}
	return d.Value.Format(time.RFC3339)
}

func (d NullDateTime) Format(format string) string {
	if !d.Valid {
		return ""
	}
	return d.Value.Format(format)
}

func (d NullDateTime) MarshalJSON() ([]byte, error) {
	if !d.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(d.Value.Format(time.RFC3339))
}

func (d *NullDateTime) UnmarshalJSON(data []byte) error {
	str := ""
	err := json.Unmarshal(data, &str)
	if err != nil {
		return err
	}
	if str == "" {
		d.Valid = false
		return nil
	}
	t, err := time.ParseInLocation(time.RFC3339, str, LocNY)
	if err != nil {
		return err
	}
	d.Value = t
	d.Valid = true
	return nil
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
