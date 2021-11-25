package timeline

import (
	"context"
	"sync"
	"time"

	"gitlab.com/learnt/api/pkg/logger"
)

type Marker interface {
	Key() string
	At() time.Time
}

// UpdaterDataProvider function that provides markers
type UpdaterDataProvider func(from, to time.Time) []Marker

type Updater struct {
	ctx      context.Context
	dp       UpdaterDataProvider
	markers  []Marker
	onMarker func(Marker)
	verify   chan time.Time
	done     chan bool
	mux      sync.Mutex
}

func NewUpdater(ctx context.Context, dp UpdaterDataProvider) *Updater {
	return &Updater{
		ctx:     ctx,
		dp:      dp,
		markers: make([]Marker, 0),
		verify:  make(chan time.Time),
		done:    make(chan bool),
	}
}

func (up *Updater) fetch() {
	up.markers = up.dp(time.Now(), time.Now().Add(time.Hour))
	return
}

// SetOnMarker sets callback to be called for each marker
func (up *Updater) SetOnMarker(f func(Marker)) {
	up.onMarker = f
}

func (up *Updater) clean() {
	up.mux.Lock()
	defer up.mux.Unlock()
	left := make([]Marker, 0)
	for _, marker := range up.markers {
		if marker.At().After(time.Now()) {
			left = append(left, marker)
		}
	}
	up.markers = left
}

func (up *Updater) verifier() {
	for {

		var found = make([]Marker, 0)

		for _, marker := range up.markers {
			if time.Now().After(marker.At()) {
				found = append(found, marker)
			}
		}

		up.clean()

		for _, marker := range found {
			go up.onMarker(marker)
		}

		select {
		case _ = <-up.ctx.Done():
			up.done <- true
			return
		case _ = <-up.verify:
			continue
		}
	}
}

func (up *Updater) Sync() {
	up.fetch()
	up.verify <- time.Now()
}

func (up *Updater) wait() time.Duration {

	var next time.Time

	for _, marker := range up.markers {
		if next.IsZero() || marker.At().Before(next) {
			next = marker.At().Add(0)
		}
	}

	if next.IsZero() {
		return time.Second * 10
	}

	return next.Sub(time.Now())
}

func (up *Updater) Run() <-chan bool {
	up.fetch()
	go up.verifier()
	go func() {
		for {
			up.verify <- time.Now()
			wait := up.wait()
			if wait != time.Second*10 {
				logger.Get().Debug("Updater wait ", wait)
			}
			time.Sleep(wait)
		}
	}()
	return up.done
}
