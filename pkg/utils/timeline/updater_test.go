package timeline

import (
	"context"
	"testing"
	"time"
)

type mrk struct {
	at time.Time
}

func (m mrk) Key() string {
	return ""
}

func (m mrk) At() time.Time {
	return m.at
}

func TestCreate(t *testing.T) {

	dp := func(from, to time.Time) []Marker {
		now := time.Now()
		return []Marker{
			&mrk{now.Add(time.Second * 5)},
			&mrk{now.Add(time.Second * 7)},
		}
	}

	var calls int

	ctx, cancel := context.WithCancel(context.Background())

	u := NewUpdater(ctx, dp)

	u.SetOnMarker(func(m Marker) {
		calls++
	})

	go func() {
		time.Sleep(time.Second * 8)
		cancel()
	}()

	wait := u.Run()

	if len(u.markers) != 2 {
		t.Errorf("Expected 2 markers, has %d", len(u.markers))
		return
	}

	<-wait

	if calls != 2 {
		t.Errorf("Expected 2 func calls found %d", calls)
	}
}
