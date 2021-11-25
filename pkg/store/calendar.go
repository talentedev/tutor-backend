package store

import (
	"fmt"
	"gitlab.com/learnt/api/pkg/utils/timeline"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/utils"
	"gopkg.in/mgo.v2/bson"
)

// am/pm
const (
	CalendarTimePeriodAM byte = iota
	CalendarTimePeriodPM
)

var sundayInThePast = time.Date(2018, time.April, 1, 0, 0, 0, 0, time.UTC)

const (
	// availabilityOffset is how much time is added between slipt availabilities
	availabilityOffset = time.Second
	// availabiltyBuffer is how much time is allowed between availabilities that can be ignored and considered
	// one availability
	availabiltyBuffer = 5 * availabilityOffset
)

type CalendarTimeEntry struct {
	Hour   int  `json:"hour" bson:"hour"`
	Minute int  `json:"minute" bson:"minute"`
	Period byte `json:"period" bson:"period"`
	Full   Time `json:"full" bson:"full"`
}

func (cte *CalendarTimeEntry) String() string {
	var p string
	switch cte.Period {
	case CalendarTimePeriodAM:
		p = "am"
	case CalendarTimePeriodPM:
		p = "pm"
	}
	return fmt.Sprintf("%d:%02d %s", cte.Hour, cte.Minute, p)
}

func (cte *CalendarTimeEntry) FromString(t string) error {
	if !utils.IsStringTime(t, true) {
		return fmt.Errorf("invalid time")
	}

	sepPos := strings.Index(t, ":")
	// jump over error handling as the validator already does it
	cte.Hour, _ = strconv.Atoi(t[0:sepPos])
	cte.Minute, _ = strconv.Atoi(t[sepPos+1 : sepPos+3])

	hour := cte.Hour
	switch t[len(t)-2:] {
	case "am":
		cte.Period = CalendarTimePeriodAM
	case "pm":
		hour += 12
		cte.Period = CalendarTimePeriodPM
	}

	cte.Full = Time{Hour: hour, Minute: cte.Minute}

	return nil
}

func (cte *CalendarTimeEntry) FromTime(h, m int) error {
	if h < 1 || h > 24 { // todo: check
		return fmt.Errorf("invalid hour")
	}

	if m < 0 || m > 59 {
		return fmt.Errorf("invalid minute")
	}

	if h > 12 {
		cte.Hour = h - 12
		cte.Period = CalendarTimePeriodPM
		cte.Full.Hour = h
	} else {
		cte.Hour = h
		cte.Period = CalendarTimePeriodAM
		cte.Full.Hour = h
	}

	cte.Minute = m
	cte.Full.Minute = m

	return nil
}

func comparableDay(a AvailabilitySlot, futureSunday bool) (from, to time.Time) {
	duration := a.To.Sub(a.From)
	// Make dates relative to a Sunday in the past so adding the int of the weekday will keep the day consistent
	dayAddition := int(a.From.Weekday())
	// If a date spans from Saturday to Sunday in either of comparable dates the sunday needs to be the next week
	if dayAddition == int(time.Sunday) && futureSunday {
		dayAddition += 7
	}
	from = time.Date(
		sundayInThePast.Year(),
		sundayInThePast.Month(),
		sundayInThePast.Day()+dayAddition,
		a.From.Hour(),
		a.From.Minute(),
		a.From.Second(),
		a.From.Nanosecond(),
		a.From.Location(),
	)
	return from, from.Add(duration)
}

func comparableDays(a, b AvailabilitySlot) (aFrom, aTo, bFrom, bTo time.Time) {
	futureSunday := false
	if a.From.Weekday() > a.To.Weekday() || b.From.Weekday() > b.To.Weekday() {
		futureSunday = true
	}
	aFrom, aTo = comparableDay(a, futureSunday)
	bFrom, bTo = comparableDay(b, futureSunday)
	return
}

// Get availability - lessons
func (u *UserMgo) GetAvailability(recurrent bool, includeLessons bool) (availability *timeline.Availability) {
	if u.Tutoring == nil || u.Tutoring.Availability == nil {
		return availability
	}

	if includeLessons {
		var lessons = u.GetLessonsTimelineSlots()
		return u.Tutoring.Availability.GetTimeline(recurrent, lessons...)
	}
	// var lessons = u.GetLessonsTimelineSlots()

	// u.Tutoring.Availability.SetLocation(u.TimezoneLocation())

	return u.Tutoring.Availability.GetTimeline(recurrent)
}

func (u *UserMgo) GetAvailabilityWithBlackout(recurrent bool, blackout ...timeline.SlotProvider) (availability *timeline.Availability) {

	if u.Tutoring == nil || u.Tutoring.Availability == nil {
		return availability
	}

	// var lessons = u.GetLessonsTimelineSlots()

	// u.Tutoring.Availability.SetLocation(u.TimezoneLocation())

	return u.Tutoring.Availability.GetTimeline(recurrent, blackout...)
}

// Get blackout - lessons
func (u *UserMgo) GetBlackout(recurrent bool) (availability *timeline.Availability) {
	if u.Tutoring == nil || u.Tutoring.Blackout == nil {
		return availability
	}

	return u.Tutoring.Blackout.GetTimeline(recurrent)
}

func (u *UserMgo) IsAvailable(from, to time.Time, recurrentOny bool) bool {

	if u.Tutoring == nil || u.Tutoring.Availability == nil {
		return false
	}

	// This gets availability and excludes availability if lesson is booked for time slot
	av := u.GetAvailability(recurrentOny, true)

	if recurrentOny {
		return av.IsAvailableRecurrent(from, to)
	}

	return av.IsAvailable(from, to)
}

// AddAvailability adds availability to a tutor
func (u *UserMgo) AddAvailability(slot *AvailabilitySlot, recurrent bool) error {

	if !recurrent && (slot.From.Before(time.Now()) || slot.To.Before(time.Now())) {
		return fmt.Errorf("availability can't be in the past")
	}

	if !u.IsTutor() {
		return fmt.Errorf("user is not a tutor")
	}

	if u.Tutoring.Availability == nil {
		u.Tutoring.Availability = &Availability{}
	}

	if err := u.Tutoring.Availability.Add(slot, recurrent); err != nil {
		return errors.Wrap(err, "Failed to add availability")
	}

	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{
		"tutoring.availability": u.Tutoring.Availability,
	}})

}

// RemoveAvailability removes availability to a tutor
func (u *UserMgo) RemoveAvailability(id bson.ObjectId) error {
	if !u.IsTutor() {
		return fmt.Errorf("user is not a tutor")
	}

	if u.Tutoring.Availability == nil {
		return nil
	}

	if err := u.Tutoring.Availability.Remove(id); err != nil {
		return errors.Wrap(err, "Failed to remove availability")
	}

	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{
		"tutoring.availability": u.Tutoring.Availability,
	}})
}

// AddAvailability adds availability to a tutor
func (u *UserMgo) AddBlackout(slot *AvailabilitySlot, recurrent bool) error {

	if !recurrent && (slot.From.Before(time.Now()) || slot.To.Before(time.Now())) {
		return fmt.Errorf("blackout can't be in the past")
	}

	if !u.IsTutor() {
		return fmt.Errorf("user is not a tutor")
	}

	if u.Tutoring.Blackout == nil {
		u.Tutoring.Blackout = &Availability{}
	}

	if err := u.Tutoring.Blackout.Add(slot, recurrent); err != nil {
		return errors.Wrap(err, "Failed to add availability")
	}

	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{
		"tutoring.blackout": u.Tutoring.Blackout,
	}})

}

// RemoveAvailability removes availability to a tutor
func (u *UserMgo) RemoveBlackout(id bson.ObjectId) error {
	if !u.IsTutor() {
		return fmt.Errorf("user is not a tutor")
	}

	if u.Tutoring.Blackout == nil {
		return nil
	}

	if err := u.Tutoring.Blackout.Remove(id); err != nil {
		return errors.Wrap(err, "Failed to remove availability")
	}

	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{
		"tutoring.blackout": u.Tutoring.Blackout,
	}})
}

// SetAvailabilityRecurrency updates an availability to be recurrent or not
func (u *UserMgo) SetBlackoutRecurrency(id bson.ObjectId, recurrent bool) error {
	if !u.IsTutor() {
		return fmt.Errorf("user is not a tutor")
	}

	if u.Tutoring.Blackout == nil {
		return nil
	}

	if err := u.Tutoring.Blackout.Update(id, recurrent); err != nil {
		return errors.Wrap(err, "Failed to update availability")
	}

	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{
		"tutoring.availability": u.Tutoring.Blackout,
	}})
}

// SetAvailabilityRecurrency updates an availability to be recurrent or not
func (u *UserMgo) SetAvailabilityRecurrency(id bson.ObjectId, recurrent bool) error {
	if !u.IsTutor() {
		return fmt.Errorf("user is not a tutor")
	}

	if u.Tutoring.Availability == nil {
		return nil
	}

	if err := u.Tutoring.Availability.Update(id, recurrent); err != nil {
		return errors.Wrap(err, "Failed to update availability")
	}

	return GetCollection("users").UpdateId(u.ID, bson.M{"$set": bson.M{
		"tutoring.availability": u.Tutoring.Availability,
	}})
}
