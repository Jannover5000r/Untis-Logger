package Untis

import "time"

var Location *time.Location

func GetTime() time.Time {
	return time.Now().In(Location)
}
