// datetime
package main

import "time"

// remove the time component of a datetime to get just a date at 00:00:00
func truncDate(t time.Time) time.Time {
	hour, min, sec := t.Clock()
	nano := t.Nanosecond()

	d := time.Duration(0) - (time.Duration(nano) + time.Duration(sec)*time.Second + time.Duration(min)*time.Minute + time.Duration(hour)*time.Hour)
	return t.Add(d)
}

func toDateTime(d string, loc *time.Location) time.Time {
	t, _ := time.ParseInLocation(time.RFC3339, d, loc)
	return t
}
