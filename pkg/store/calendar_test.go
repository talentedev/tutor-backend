package store

import (
	"testing"
	"time"
)

func TestComparableDay(t *testing.T) {
	tests := []struct {
		name string
		from string
		to   string
	}{
		{
			name: "Saturday to Sunday",
			from: "2019-03-02T22:21:17Z",
			to:   "2019-03-03T04:28:38Z",
		},
		{
			name: "Sunday to Monday",
			from: "2019-03-03T22:21:17Z",
			to:   "2019-03-04T00:28:38Z",
		},
		{
			name: "Monday to Sunday",
			from: "2019-03-04T22:21:17Z",
			to:   "2019-03-10T00:28:38Z",
		},
		{
			name: "Same day",
			from: "2019-03-04T00:28:38Z",
			to:   "2019-03-04T04:28:38Z",
		},
		{
			name: "now",
			from: time.Now().Format(time.RFC3339),
			to:   time.Now().Add(2 * time.Hour).Format(time.RFC3339),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			from, err := time.Parse(time.RFC3339, test.from)
			if err != nil {
				t.Fatal("could not parse 'from' time")
			}
			to, err := time.Parse(time.RFC3339, test.to)
			if err != nil {
				t.Fatalf("could not parse 'to' time")
			}

			cFrom, cTo := comparableDay(AvailabilitySlot{From: from, To: to}, false)

			if from.Weekday() != cFrom.Weekday() {
				t.Fatalf("'from' weekday did not match %s-%s", from.Weekday(), cFrom.Weekday())
			}
			if to.Weekday() != cTo.Weekday() {
				t.Fatalf("'to' weekday did not match %s-%s", to.Weekday(), cTo.Weekday())
			}
			duration, cDuration := to.Sub(from), cTo.Sub(cFrom)
			if duration != cDuration {
				t.Fatalf("duration after conversion did not match %s-%s", duration, cDuration)
			}
		})
	}
}
