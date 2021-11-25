package store

import (
	"fmt"
	"strconv"
	"time"
)

func zerofill(n int) string {
	if n < 10 {
		return fmt.Sprint("0", strconv.Itoa(n))
	}
	return strconv.Itoa(n)
}

type Time struct {
	Hour   int       `json:"hour" bson:"hour"`
	Minute int       `json:"minute" bson:"minute"`
	Date   time.Time `json:"date,omitempty" bson:"date,omitempty"`
}

func (t *Time) Time() time.Time {
	y, m, d := t.Date.Date()
	return time.Date(y, m, d, t.Hour, t.Minute, 0, 0, time.UTC)
}

func (t *Time) Duration() time.Duration {
	return time.Duration(t.Hour)*time.Hour + time.Duration(t.Minute)*time.Minute
}

func (t *Time) String() string {
	return zerofill(t.Hour) + ":" + zerofill(t.Minute)
}

type TimeRange struct {
	Min Time `json:"min" bson:"min"`
	Max Time `json:"max" bson:"max"`
}
