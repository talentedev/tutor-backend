package utils

import (
	"fmt"
	"math"
	"time"

	"gitlab.com/learnt/api/pkg/logger"
)

const MMDDYY_TIME = "01/02/2006 3:04pm"

// FormatTime formats the specified time in colloquial English.
func FormatTime(t time.Time) string {
	n := time.Now()

	if t.YearDay() == n.YearDay() {
		return fmt.Sprintf("Today at %s", t.Format("3:04pm"))
	}

	if t.YearDay() == n.YearDay()+1 {
		return fmt.Sprintf("Tomorrow at %s", t.Format("3:04pm"))
	}

	if t.Year() == n.Year() {
		return t.Format("at 02 Jan 3:04pm")
	}

	return t.Format("at 02 Jan 2006 3:04pm")
}

// FormatTime formats the specified time in colloquial English.
func FormatTimeWithTimezone(t time.Time, timezone string) string {
	formattedTime := t

	loc, err := time.LoadLocation(timezone)
	if err == nil {
		formattedTime = formattedTime.In(loc)
	} else {
		logger.Get().Errorf("timezone conversion error. timezone: %s | err: %+v", timezone, err)
	}

	return FormatTime(formattedTime)
}

// GeneralFormatTime formats the specified time in colloquial English with date format "mm/dd/yyyy at <time>".
func GeneralFormatTime(format string, t time.Time) string {
	if len(format) == 0 {
		return t.Format(MMDDYY_TIME)
	}

	return t.Format(format)
}

// DateIsBefore checks if the second field is before the first field.
func DateIsBefore(to, from time.Time) bool {
	return to.Sub(from).Nanoseconds() < 0
}

// DateIsBeforeOrEqual checks if the second field is before or equal to the first field.
func DateIsBeforeOrEqual(to, from time.Time) bool {
	return to.Sub(from).Nanoseconds() <= 0
}

// DateIsAfter checks if the second field is after the first field.
func DateIsAfter(to, from time.Time) bool {
	return to.Sub(from).Nanoseconds() > 0
}

// DateIsAfterOrEqual checks if the second field is after or equal to the first field.
func DateIsAfterOrEqual(to, from time.Time) bool {
	return to.Sub(from).Nanoseconds() >= 0
}

// DateIsBetween checks if the provided time is between the two references.
func DateIsBetween(t, oldRef, newRef time.Time) bool {
	return DateIsAfter(t, oldRef) && DateIsBefore(t, newRef)
}

// DateIsBetweenOrEqual checks if the provided time is between the two references or equal to them.
func DateIsBetweenOrEqual(t, oldRef, newRef time.Time) bool {
	return DateIsAfterOrEqual(t, oldRef) && DateIsBeforeOrEqual(t, newRef)
}

// DateIsEqual returns whether the provided time.Times are equal up to minutes.
func DateIsEqual(a, b time.Time) bool {
	h1, m1 := a.Hour(), a.Minute()
	h2, m2 := b.Hour(), b.Minute()
	return DateIsSame(a, b) && h1 == h2 && m1 == m2
}

// DateIsSame returns whether the provided time.Times have the same calendar day
func DateIsSame(a, b time.Time) bool {
	y1, m1, d1 := a.Date()
	y2, m2, d2 := b.Date()
	return y1 == y2 && m1 == m2 && d1 == d2
}

func DateLessThanEqualDaysAgo(t time.Time, x int) bool {
	return math.Ceil(time.Now().Sub(t).Hours()/24.0) <= float64(x)
}

// AbsDuration returns the positive alue of a duration
func AbsDuration(d time.Duration) time.Duration {
	if d < 0 {
		return -1 * d
	}
	return d
}

func TimeInCurrentWeek(u time.Time) bool {
	now := time.Now()
	startOfWeek := now.AddDate(0, 0, -int(now.Weekday()))
	endOfWeek := now.AddDate(0, 0, int(time.Saturday-now.Weekday()))
	if startOfWeek.Before(u) && endOfWeek.After(u) {
		return true
	}
	return false
}

func TimeNextWeek(u time.Time) time.Time {
	for i := 0; i < 7; i++ {
		u = u.Add(time.Hour * 24)
	}
	return u
}

func TimeBasicShortDateTimeFormat(u time.Time) string {
	return u.Format("Jan 02, 2006 15:04:05")
}
