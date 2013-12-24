// datetime
package main

import "time"

func toDateTime(d string, loc *time.Location) (t time.Time, err error) {
	return time.ParseInLocation(time.RFC3339, d, loc)
}
