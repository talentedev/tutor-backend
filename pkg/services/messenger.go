package services

import (
	"context"
	"gitlab.com/learnt/api/pkg/ws"
)

var observer *ThreadObserver

type ThreadObserver struct {
	engine  *ws.Engine
	mux     MutexGroup
	threads map[string][]string
}

func InitMessenger(ctx context.Context) {
	observer = &ThreadObserver{
		engine:  ws.GetEngine(),
		mux:     MutexGroup{},
		threads: make(map[string][]string, 0),
	}
	observer.Listen("thread.observe", observer.ObserveThread)
	observer.Listen("thread.unobserve", observer.UnobserveThread)
}

func (observer *ThreadObserver) Listen(event string, handler func(event ws.Event)) {
	observer.engine.Listen(event, func(event ws.Event, engine *ws.Engine) {
		handler(event)
	})
}

func (observer *ThreadObserver) ObserveThread(event ws.Event) {
	user := event.Source.GetUser()
	if !user.IsAdmin() {
		return
	}
	thread := event.Data["thread"].(string)
	observer.threads[thread] = append(observer.threads[thread], user.ID.Hex())
}

func (observer *ThreadObserver) UnobserveThread(event ws.Event) {
	user := event.Source.GetUser()
	if !user.IsAdmin() {
		return
	}
	thread := event.Data["thread"].(string)
	observers := make([]string, 0)
	userId := user.ID.Hex()
	for _, observerId := range observer.threads[thread] {
		if observerId != userId {
			observers = append(observers, observerId)
		}
	}
	if len(observers) > 0 {
		observer.threads[thread] = observers
	} else {
		delete(observer.threads, thread)
	}
}

func GetThreadObservers(thread string) []string {
	return observer.threads[thread]
}
