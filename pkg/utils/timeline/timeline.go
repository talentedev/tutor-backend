package timeline

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/pkg/errors"
)

type timelineSort struct {
	Slots []SlotProvider
}

func (ts *timelineSort) Len() int {
	return len(ts.Slots)
}

func (ts *timelineSort) Less(i, j int) bool {
	return ts.Slots[i].GetFrom().Before(ts.Slots[j].GetFrom())
}

func (ts *timelineSort) Swap(i, j int) {
	ts.Slots[i], ts.Slots[j] = ts.Slots[j], ts.Slots[i]
}

type SlotProvider interface {
	GetID() string
	GetFrom() time.Time
	GetTo() time.Time
	GetOccurence() Occurence
	Equal(slot SlotProvider) bool
	SetTimezone(loc *time.Location)
}

type Occurence int

const (
	None Occurence = iota
	Weekly
	TwoWeeks
)

type Slot struct {
	ID        string    `json:"id"`
	From      time.Time `json:"from"`
	To        time.Time `json:"to"`
	Occurence Occurence `json:"occurence"`
}

func (s *Slot) GetID() string {
	return s.ID
}

func (s *Slot) SetTimezone(loc *time.Location) {
	s.From = s.From.In(loc)
	s.To = s.To.In(loc)
}
func (s Slot) GetFrom() time.Time {
	return s.From.Add(0)
}

func (s Slot) GetTo() time.Time {
	return s.To.Add(0)
}

func (s Slot) GetOccurence() Occurence {
	return s.Occurence
}

func (s Slot) Equal(v SlotProvider) bool {
	if v == nil {
		return false
	}
	return s.From.Equal(v.GetFrom()) && s.To.Equal(v.GetTo()) && s.Occurence == v.GetOccurence()
}

func toString(slot SlotProvider) string {
	return intvToString(slot.GetFrom(), slot.GetTo())
}

func intvToString(from, to time.Time) string {
	format := " [ 2006 Mon Jan 2 15:04 -0700 ] "
	return fmt.Sprint(
		from.Format(format),
		"-",
		to.Format(format),
	)
}

// Verify if t >= from && t <= to
func IsBetween(t, from, to time.Time) bool {
	return ((t.After(from) || t.Equal(from)) && t.Before(to)) || ((t.Before(to) || t.Equal(to)) && t.After(from))
}

// Verify if slot is in provided interval
// slot.from in from/to && slot.to in from/to
func SlotIn(slot SlotProvider, from, to time.Time) bool {
	return IsBetween(slot.GetFrom(), from, to) && IsBetween(slot.GetTo(), from, to)
}

// Verify if provided interval is in slot
// from in slot.from/slot.to and to in slot.from/slot.to
func InSlot(slot SlotProvider, from, to time.Time) bool {
	return IsBetween(from, slot.GetFrom(), slot.GetTo()) && IsBetween(to, slot.GetFrom(), slot.GetTo())
}

// Slot appears in interval
func SlotEnters(slot SlotProvider, from, to time.Time) bool {

	// INTV  [----]
	// SLOT [--------]
	if slot.GetFrom().Before(from) && slot.GetTo().After(to) {
		return true
	}

	// INTV [----]  [-----]
	// SLOT [----]    [--]
	if IsBetween(slot.GetFrom(), from, to) && IsBetween(slot.GetTo(), from, to) {
		return true
	}

	// INTV [----]
	// SLOT     [----]
	if slot.GetTo().After(to) && slot.GetFrom().Before(to) && (slot.GetFrom().After(from) || slot.GetFrom().Equal(from)) {
		return true
	}

	// INTV     [----]
	// SLOT [----]
	if slot.GetFrom().Before(from) && slot.GetTo().After(from) && (slot.GetTo().Before(to) || slot.GetTo().Equal(to)) {
		return true
	}

	return false
}

func Subtract(item SlotProvider, list []SlotProvider, from, to time.Time) []SlotProvider {
	for i, slot := range list {
		if InSlot(slot, item.GetFrom(), item.GetTo()) {
			list = append(list[:i], list[i+1:]...)
			chunks := split(slot, item)
			for _, chunk := range chunks {
				if SlotEnters(chunk, from, to) {
					list = append(list, chunk)
				}
			}

			return Subtract(item, list, from, to)
		}
	}
	return list
}

// Shift slot between
func Shift(slot SlotProvider, from, to time.Time) SlotProvider {
	if slot == nil {
		return nil
	}

	if slot.GetFrom().After(to) {
		return nil
	}

	slotFrom := slot.GetFrom()
	duration := slot.GetTo().Sub(slotFrom)

	for slotFrom.Before(to) {
		slotFrom = slotFrom.AddDate(0, 0, 7)

		slot = &Slot{
			ID:        slot.GetID(),
			From:      slotFrom,
			To:        slotFrom.Add(duration),
			Occurence: slot.GetOccurence(),
		}

		if SlotEnters(slot, from, to) {
			return slot
		}
	}

	return nil
}

func split(slot SlotProvider, sub SlotProvider) (out []SlotProvider) {

	out = make([]SlotProvider, 0)

	if !InSlot(slot, sub.GetFrom(), sub.GetTo()) {
		return
	}

	diffLeft := sub.GetFrom().Sub(slot.GetFrom())
	diffRight := slot.GetTo().Sub(sub.GetTo())

	if diffLeft > 0 {
		out = append(out, &Slot{
			ID:        slot.GetID(),
			From:      slot.GetFrom().Add(0),
			To:        sub.GetFrom().Add(0),
			Occurence: slot.GetOccurence(),
		})
	}

	if diffRight > 0 {
		out = append(out, &Slot{
			ID:        slot.GetID(),
			From:      sub.GetTo().Add(0),
			To:        slot.GetTo().Add(0),
			Occurence: slot.GetOccurence(),
		})
	}

	return
}

type Timeline struct {
	slots []SlotProvider
	mux   sync.Mutex
}

func (t *Timeline) SetTimezone(loc *time.Location) {
	for _, slot := range t.slots {
		slot.SetTimezone(loc)
	}
}

func (t *Timeline) at(u time.Time) SlotProvider {
	for _, slot := range t.slots {
		if IsBetween(u, slot.GetFrom(), slot.GetTo()) {
			return slot //63713476800
			//63713480400
		}
	}
	return nil
}

func (t *Timeline) remove(rem SlotProvider) {
	for i, slot := range t.slots {
		if slot.Equal(rem) {
			t.slots = append(t.slots[:i], t.slots[i+1:]...)
			return
		}
	}
}

func (t *Timeline) Add(slots ...SlotProvider) error {

	if len(slots) == 0 {
		return errors.New("No slots to add")
	}

	if t.slots == nil {
		t.slots = make([]SlotProvider, 0)
	}

	t.mux.Lock()
	defer t.mux.Unlock()

	for _, slot := range slots {

		if !slot.GetTo().After(slot.GetFrom()) {
			return errors.New("Invalid slot")
		}

		from := t.at(slot.GetFrom())
		to := t.at(slot.GetTo())

		if from != nil && to != nil && from.GetOccurence() != slot.GetOccurence() {
			return errors.New("Slot found with different occurence")
		}

		// Slot already exist
		if from != nil && from.Equal(to) {
			return errors.New("Slot already exists")
		}

		if from != nil && to == nil {

			if from.GetOccurence() != slot.GetOccurence() {
				return errors.New(
					"Can't merge slot, occurence mismatching",
				)
			}

			t.remove(from)
			t.slots = append(t.slots, &Slot{
				ID:        slot.GetID(),
				From:      from.GetFrom().Add(0),
				To:        slot.GetTo().Add(0),
				Occurence: slot.GetOccurence(),
			})

			continue
		}

		if from == nil && to != nil {

			if to.GetOccurence() != slot.GetOccurence() {
				return errors.New(
					"Can't merge slot, occurence mismatching",
				)
			}

			t.remove(to)
			t.slots = append(t.slots, &Slot{
				ID:        slot.GetID(),
				From:      slot.GetFrom().Add(0),
				To:        to.GetTo().Add(0),
				Occurence: slot.GetOccurence(),
			})
			continue
		}

		if from == nil && to == nil {
			t.slots = append(t.slots, slot)
		}
	}

	t.Sort()

	return nil
}

func (t *Timeline) Len() int {
	return len(t.slots)
}

func (t *Timeline) Get(from, to time.Time, diff ...SlotProvider) (out []SlotProvider, err error) {

	if from.After(to) {
		return out, errors.New("Invalid interval")
	}

	out = make([]SlotProvider, 0)

	for _, slot := range t.slots {

		// Skip if slot is after range
		if slot.GetFrom().After(to) {
			continue
		}

		if slot.GetOccurence() == None && SlotEnters(slot, from, to) {
			out = append(out, slot)
			continue
		}

		if slot.GetOccurence() == Weekly {

			if SlotEnters(slot, from, to) {
				out = append(out, slot)
			}

			slot = Shift(slot, from, to)

			for slot != nil {
				if SlotEnters(slot, from, to) {
					out = append(out, slot)
				}
				slot = Shift(slot, from, to)
			}
		}
	}

	if len(out) == 0 || len(diff) == 0 {
		return out, nil
	}

	for _, sub := range diff {

		if sub.GetOccurence() == Weekly {

			if SlotEnters(sub, from, to) {
				out = Subtract(sub, out, from, to)
			}

			sub = Shift(sub, from, to)

			for sub != nil {
				out = Subtract(sub, out, from, to)
				sub = Shift(sub, from, to)
			}

			continue
		}

		out = Subtract(sub, out, from, to)
	}

	return
}

func (t *Timeline) Sort() {
	srt := &timelineSort{t.slots}
	sort.Sort(srt)
	t.slots = srt.Slots
}

func (t *Timeline) GetAvailability(booked ...SlotProvider) *Availability {
	return &Availability{
		timeline: t,
		booked:   booked,
	}
}

type Availability struct {
	timeline *Timeline
	booked   []SlotProvider
}

func (a *Availability) Get(from, to time.Time) (out []SlotProvider, err error) {
	return a.timeline.Get(from, to, a.booked...)
}

// verify if recurrent available
func (a *Availability) IsAvailableRecurrent(from, to time.Time) bool {

	if len(a.booked) == 0 {

		slots, _ := a.Get(from, to)

		if len(slots) == 0 || slots[0].GetOccurence() == None {
			return false
		}

		if !InSlot(slots[0], from, to) {
			return false
		}

		return true

	}

	var count = 0

	var lastBookedTime time.Time

	for _, b := range a.booked {
		if lastBookedTime.IsZero() || b.GetTo().After(lastBookedTime) {
			lastBookedTime = b.GetTo().Add(0)
		}
	}

	// iterate next 5 months
	for from.Before(lastBookedTime) {

		slots, _ := a.Get(from, to)

		if len(slots) == 0 {
			return false
		}

		for _, slot := range slots {
			if !InSlot(slot, from, to) {
				return false
			}
		}

		if count > 100 {
			panic("Unexpected")
		}

		// next week shift
		from = from.AddDate(0, 0, 7)
		to = to.AddDate(0, 0, 7)
		count++
	}

	return true
}

func (a *Availability) IsAvailable(from, to time.Time) bool {

	slots, _ := a.Get(from.In(time.UTC), to.In(time.UTC))

	if len(slots) == 0 || !InSlot(slots[0], from, to) {
		return false
	}

	return true
}
