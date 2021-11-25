package store

import (
	"gitlab.com/learnt/api/pkg/utils/timeline"
	"sync"
	"time"

	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
)

// Availability lets a tutor specify when they are available to take lessons
type AvailabilitySlot struct {
	ID   bson.ObjectId `json:"_id" bson:"_id"`
	From time.Time     `json:"from" bson:"from"`
	To   time.Time     `json:"to" bson:"to"`
}

func (slot *AvailabilitySlot) SetLocation(loc *time.Location) {
	slot.From = slot.From.In(loc)
	slot.To = slot.To.In(loc)
}

type Availability struct {
	Slots     []*AvailabilitySlot `json:"slots" bson:"slots"`
	Recurrent []*AvailabilitySlot `json:"recurrent" bson:"recurrent"`
	mux       sync.Mutex
}

func (a *Availability) SetLocation(loc *time.Location) {

	for _, slot := range a.Slots {
		slot.SetLocation(loc)
	}

	for _, slot := range a.Recurrent {
		slot.SetLocation(loc)
	}
}

func (a *Availability) Clone() *Availability {
	return &Availability{
		Slots:     a.Slots,
		Recurrent: a.Recurrent,
	}
}

func (a *Availability) Update(id bson.ObjectId, recurrent bool) error {

	for _, slot := range a.Slots {
		if slot.ID.Hex() == id.Hex() {

			if !recurrent {
				return errors.New("Slot already non recurrent")
			}

			tmp := slot
			a.Remove(id)
			return a.Add(tmp, true)
		}
	}

	for _, slot := range a.Recurrent {
		if slot.ID.Hex() == id.Hex() {

			if recurrent {
				return errors.New("Slot already recurrent")
			}

			tmp := slot
			a.Remove(id)
			return a.Add(tmp, false)
		}
	}

	return errors.Errorf("Slot with id %s not found", id)
}

func (a *Availability) Remove(id bson.ObjectId) error {

	var removed bool

	a.mux.Lock()
	defer a.mux.Unlock()

	for index, slot := range a.Slots {
		if slot.ID.Hex() == id.Hex() {
			a.Slots = append(a.Slots[:index], a.Slots[index+1:]...)
			removed = true
			break
		}
	}

	for index, slot := range a.Recurrent {
		if slot.ID.Hex() == id.Hex() {
			a.Recurrent = append(a.Recurrent[:index], a.Recurrent[index+1:]...)
			removed = true
			break
		}
	}

	if !removed {
		return errors.New("No slot to remove")
	}

	return nil
}

func (a *Availability) Add(slot *AvailabilitySlot, recurrent bool) error {

	slot.ID = bson.NewObjectId()

	if recurrent {
		a.Recurrent = append(a.Recurrent, slot)
	} else {
		a.Slots = append(a.Slots, slot)
	}

	return nil
}

func (a *Availability) GetTimeline(recurrent bool, booked ...timeline.SlotProvider) *timeline.Availability {
	tl := timeline.Timeline{}
	if a.Recurrent != nil {
		for _, slot := range a.Recurrent {
			_ = tl.Add(&timeline.Slot{
				ID:        slot.ID.Hex(),
				From:      slot.From.Add(0),
				To:        slot.To.Add(0),
				Occurence: timeline.Weekly,
			})
		}
	}

	if recurrent {
		return tl.GetAvailability(booked...)
	}

	if a.Slots != nil {
		for _, slot := range a.Slots {
			_ = tl.Add(&timeline.Slot{
				ID:        slot.ID.Hex(),
				From:      slot.From.Add(0),
				To:        slot.To.Add(0),
				Occurence: timeline.None,
			})
		}
	}

	return tl.GetAvailability(booked...)
}
