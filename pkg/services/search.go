package services

import (
	"regexp"
	"sort"
	"strconv"
	"time"

	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/projections"

	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"
	"gopkg.in/mgo.v2/bson"
)

const metersPerMile = 1609.34

type search struct {
	match          bson.M
	timezone       string
	availabilities []store.AvailabilitySlot
}

var s *search

// GetSearch returns the search instance.
func GetSearch() *search {
	if s == nil {
		s = &search{}
	}
	return s
}

// Clear will reset the search filters to the default ones.
func (s *search) Clear() {
	s.match = bson.M{
		"disabled": false,
		"role":     bson.M{"$eq": store.RoleTutor},

		"profile.avatar": bson.M{"$exists": true},
		"tutoring.rate":  bson.M{"$gt": 0},
		"approval":       store.ApprovalStatusApproved,

		// LRNT-96--Sam--10/8/2020 -- commenting out for launch. only need activated tutor with picture and payments
		// "tutoring.rate":                   bson.M{"$exists": true},
		// "tutoring.subjects.0":             bson.M{"$exists": true},
		// "tutoring.degrees.0":              bson.M{"$exists": true},
		// "tutoring.availability.slots":     bson.M{"$exists": true},
		// "tutoring.availability.recurrent": bson.M{"$exists": true},

		"payments.connect": bson.M{"$exists": true, "$not": bson.M{"$size": 0}},
	}

	s.availabilities = make([]store.AvailabilitySlot, 0)
}

// Timezone sets the timezone for time filtering.
func (s *search) Timezone(t string) *search {
	s.timezone = t
	return s
}

// ExcludeLoggedUser will remove the user who searched from the set of results.
func (s *search) ExcludeLoggedUser(user *store.UserMgo) *search {
	s.match["_id"] = bson.M{"$ne": user.ID}
	return s
}

// OnlineFilter will search for online or offline users.
func (s *search) OnlineFilter(online bool) *search {
	if online {
		s.match["online"] = 1
	} else {
		s.match["online"] = 0
	}
	return s
}

// InstantBook sets the instant booking flag for the users.
func (s *search) InstantBook(active bool) *search {
	s.match["tutoring.instant_booking"] = active
	return s
}

// InstantSession sets the instant session flag for the online users.
func (s *search) InstantSession(active bool) *search {
	s.match["tutoring.instant_session"] = active
	s.match["online"] = 1
	return s
}

// Price sets the price range for the tutor's rate per hour.
func (s *search) Price(min, max int) *search {
	s.match["tutoring.rate"] = bson.M{
		"$exists": true,
		"$gte":    min,
		"$lte":    max,
	}
	return s
}

// ExcludeTestAccounts excludes test accounts in query.
// FIXME: this doesn't work with search as expected
func (s *search) ExcludeTestAccounts() *search {
	s.match["is_test_account"] = bson.M{
		"$eq": false,
	}

	return s
}

// MeetLocation sets the meeting coordinates.
func (s *search) MeetLocation(coordinates *store.Coordinates) *search {

	var searchMiles float64 = 50

	if config.GetConfig().IsSet("app.search_miles") {
		searchMiles = config.GetConfig().GetFloat64("app.search_miles")
	}

	s.match["location.position"] = &bson.M{
		"$geoNear": bson.M{
			"$geometry": bson.M{
				"type":        "Point",
				"coordinates": []float64{coordinates.Lng, coordinates.Lat},
			},
			"$maxDistance": searchMiles * metersPerMile,
		},
	}
	return s
}

// getWeekDates returns dates for week days based on the availability
func getWeekDates(availability store.GeneralAvailability) []time.Time {
	now := time.Now()
	dateRange := 14 // how many days in the future we search for availability

	switch availability {
	case store.MondayMorning, store.MondayAfternoon, store.MondayEvening, store.MondayAllDay,
		store.MondayFirstHalf, store.MondaySecondHalf, store.MondayEnds:
		return getDatesForWeekday(now, time.Monday, dateRange)
	case store.TuesdayMorning, store.TuesdayAfternoon, store.TuesdayEvening, store.TuesdayAllDay,
		store.TuesdayFirstHalf, store.TuesdaySecondHalf, store.TuesdayEnds:
		return getDatesForWeekday(now, time.Tuesday, dateRange)
	case store.WednesdayMorning, store.WednesdayAfternoon, store.WednesdayEvening, store.WednesdayAllDay,
		store.WednesdayFirstHalf, store.WednesdaySecondHalf, store.WednesdayEnds:
		return getDatesForWeekday(now, time.Wednesday, dateRange)
	case store.ThursdayMorning, store.ThursdayAfternoon, store.ThursdayEvening, store.ThursdayAllDay,
		store.ThursdayFirstHalf, store.ThursdaySecondHalf, store.ThursdayEnds:
		return getDatesForWeekday(now, time.Thursday, dateRange)
	case store.FridayMorning, store.FridayAfternoon, store.FridayEvening, store.FridayAllDay,
		store.FridayFirstHalf, store.FridaySecondHalf, store.FridayEnds:
		return getDatesForWeekday(now, time.Friday, dateRange)
	case store.SaturdayMorning, store.SaturdayAfternoon, store.SaturdayEvening, store.SaturdayAllDay,
		store.SaturdayFirstHalf, store.SaturdaySecondHalf, store.SaturdayEnds:
		return getDatesForWeekday(now, time.Saturday, dateRange)
	case store.SundayMorning, store.SundayAfternoon, store.SundayEvening, store.SundayAllDay,
		store.SundayFirstHalf, store.SundaySecondHalf, store.SundayEnds:
		return getDatesForWeekday(now, time.Sunday, dateRange)
	default:
		return nil
	}
}

// getTimeRanges returns the time range based on the availability
func getTimeRanges(availability store.GeneralAvailability) [][2]int {
	morningRange := [2]int{0, 11}
	afternoonRange := [2]int{11, 17}
	eveningRange := [2]int{17, 23}
	allDayRange := [2]int{0, 23}

	firstHalfRange := [2]int{morningRange[0], afternoonRange[1]}
	secondHalfRange := [2]int{afternoonRange[0], eveningRange[1]}

	ranges := make([][2]int, 0)

	switch availability {
	case store.MondayMorning, store.TuesdayMorning, store.WednesdayMorning,
		store.ThursdayMorning, store.FridayMorning, store.SaturdayMorning,
		store.SundayMorning:
		ranges = append(ranges, morningRange)
	case store.MondayAfternoon, store.TuesdayAfternoon, store.WednesdayAfternoon,
		store.ThursdayAfternoon, store.FridayAfternoon, store.SaturdayAfternoon,
		store.SundayAfternoon:
		ranges = append(ranges, afternoonRange)
	case store.MondayEvening, store.TuesdayEvening, store.WednesdayEvening,
		store.ThursdayEvening, store.FridayEvening, store.SaturdayEvening,
		store.SundayEvening:
		ranges = append(ranges, eveningRange)
	case store.MondayAllDay, store.TuesdayAllDay, store.WednesdayAllDay,
		store.ThursdayAllDay, store.FridayAllDay, store.SaturdayAllDay,
		store.SundayAllDay:
		ranges = append(ranges, allDayRange)
	case store.MondayFirstHalf, store.TuesdayFirstHalf, store.WednesdayFirstHalf,
		store.ThursdayFirstHalf, store.FridayFirstHalf, store.SaturdayFirstHalf, store.SundayFirstHalf:
		ranges = append(ranges, firstHalfRange)
	case store.MondaySecondHalf, store.TuesdaySecondHalf, store.WednesdaySecondHalf,
		store.ThursdaySecondHalf, store.FridaySecondHalf, store.SaturdaySecondHalf, store.SundaySecondHalf:
		ranges = append(ranges, secondHalfRange)
	case store.MondayEnds, store.TuesdayEnds, store.WednesdayEnds,
		store.ThursdayEnds, store.FridayEnds, store.SaturdayEnds, store.SundayEnds:
		ranges = append(ranges, morningRange)
		ranges = append(ranges, eveningRange)
	default:
		ranges = append(ranges, allDayRange)
	}

	return ranges
}

type generalTime struct {
	Range [][2]int
	Dates []time.Time
}

// GeneralAvailability searches for users who are available at predefined time ranges.
func (s *search) GeneralAvailability(availabilities []store.GeneralAvailability) *search {
	generalTimes := make([]generalTime, 0)

	for _, availability := range availabilities {
		generalTimes = append(generalTimes, generalTime{
			Range: getTimeRanges(availability),
			Dates: getWeekDates(availability),
		})
	}

	searchTimezone, err := time.LoadLocation(s.timezone)
	if err != nil {
		searchTimezone = time.UTC
	}

	for _, gt := range generalTimes {
		for _, r := range gt.Range {
			for _, d := range gt.Dates {
				ye, mo, da := d.Date()
				from := time.Date(ye, mo, da, r[0], 0, 0, 0, searchTimezone)
				to := time.Date(ye, mo, da, r[1], 0, 0, 0, searchTimezone)

				s.availabilities = append(s.availabilities, store.AvailabilitySlot{From: from, To: to})
			}
		}
	}

	return s
}

func getDatesForWeekday(date time.Time, wd time.Weekday, days int) []time.Time {
	if days < 0 {
		days = 7
	}
	times := make([]time.Time, 0)
	for i := 1; i <= int(days); i++ {
		date = date.AddDate(0, 0, 1)
		if date.Weekday() != wd {
			continue
		}
		times = append(times, date)
	}
	return times
}

// SpecificAvailability searches for users who are available at a specific date and time range.
func (s *search) SpecificAvailability(av string) *search {
	// Format: year-month-day_from:time_to:time
	if !utils.IsSpecificTime(av) {
		return s
	}

	date, err := time.Parse("2006-01-02", av[0:10])
	if err != nil {
		return s
	}

	fromTime, err := specificToClock(av[11:16])
	if err != nil {
		return s
	}

	toTime, err := specificToClock(av[17:])
	if err != nil {
		return s
	}

	loc, err := time.LoadLocation(s.timezone)
	if err != nil {
		loc = time.UTC
	}

	y, m, d := date.Date()
	from := time.Date(y, m, d, fromTime.Full.Hour, fromTime.Minute, 0, 0, loc)
	to := time.Date(y, m, d, toTime.Full.Hour, toTime.Minute, 0, 0, loc)

	s.availabilities = append(s.availabilities, store.AvailabilitySlot{
		From: from,
		To:   to,
	})

	return s
}

func specificToClock(s string) (*store.CalendarTimeEntry, error) {
	hour, err := strconv.Atoi(s[0:2])
	if err != nil {
		return nil, errors.Wrap(err, "couldn't parse int for hour")
	}

	minute, err := strconv.Atoi(s[3:])
	if err != nil {
		return nil, errors.Wrap(err, "couldn't parse int for minute")
	}

	timeEntry := &store.CalendarTimeEntry{}
	timeEntry.FromTime(hour, minute)

	return timeEntry, nil
}

// SubjectFilter searches for a specific subject.
func (s *search) SubjectFilter(subject bson.ObjectId) *search {
	s.match["tutoring.subjects.subject"] = subject
	return s
}

// Query searches throughout the profile for the specified query string.
func (s *search) Query(q string) *search {
	_, err := regexp.Compile(q)
	if err != nil {
		return s
	}

	query := []bson.M{
		{"profile.about": bson.M{
			"$regex": bson.RegEx{
				Pattern: q,
				Options: "gi",
			},
		}},
		{"profile.first_name": bson.M{
			"$regex": bson.RegEx{
				Pattern: q,
				Options: "gi",
			},
		}},
		{"profile.last_name": bson.M{
			"$regex": bson.RegEx{
				Pattern: q,
				Options: "gi",
			},
		}},
	}

	s.match["$or"] = query

	var subjects []store.Subject
	subjectRegExp := bson.M{"subject": bson.RegEx{Pattern: q, Options: "gi"}}
	err = store.GetCollection("subjects").Find(subjectRegExp).All(&subjects)
	if len(subjects) == 0 || err != nil {
		return s
	}

	subjectsIDs := make([]bson.ObjectId, 0)
	for _, s := range subjects {
		subjectsIDs = append(subjectsIDs, s.ID)
	}

	query = append(query, bson.M{"tutoring.subjects.subject": bson.M{"$in": subjectsIDs}})

	s.match["$or"] = query

	return s
}

// Do gets the results, applies time filters, and returns the results to the user.
func (s *search) Do(meetInPerson, meetOnline bool) ([]store.UserMgo, error) {
	queried := make([]store.UserMgo, 0)
	found := make([]store.UserMgo, 0)
	results := make([]store.UserMgo, 0)

	// Mongo doesn't allow an '$or' on a near search so these have to be done separately and combined together
	// https://docs.mongodb.com/manual/reference/operator/query/or/#or-and-geospatial-queries

	usersDB := store.GetCollection("users")
	publicProfiles := []bson.M{
		{"$or": []bson.M{
			{"preferences.is_private": bson.M{
				"$exists": false,
			}},
			{"preferences.is_private": bson.M{
				"$exists": true,
				"$eq":     false,
			}},
		}},
	}

	s.match["$and"] = publicProfiles

	if meetInPerson {
		// s.match["tutoring.meet"] = bson.M{"$bitsAnySet": store.MeetInPerson}
		s.match["tutoring.meet"] = bson.M{"$eq": store.MeetInPerson}
		if err := usersDB.Pipe([]bson.M{
			{"$match": s.match},
			projections.TutorPublicProjection,
		}).All(&queried); err != nil {
			return results, errors.Wrap(err, "error in position search query")
		}
		for _, u := range queried {
			found = append(found, u)
		}
	}

	if meetOnline {
		delete(s.match, "location.position")
		// s.match["tutoring.meet"] = bson.M{"$bitsAnySet": store.MeetOnline}
		s.match["tutoring.meet"] = bson.M{"$eq": store.MeetOnline}
		logger.Get().Debugf("search query %+v", s.match)
		if err := usersDB.Pipe([]bson.M{
			{"$match": s.match},
			projections.TutorPublicProjection,
		}).All(&queried); err != nil {
			return results, errors.Wrap(err, "error in online search query")
		}
		for _, u := range queried {
			found = append(found, u)
		}
	}

	if !meetInPerson && !meetOnline {
		delete(s.match, "location.position")
		delete(s.match, "tutoring.meet")
		if err := usersDB.Pipe([]bson.M{
			{"$match": s.match},
			projections.TutorPublicProjection,
		}).All(&found); err != nil {
			return results, errors.Wrap(err, "error in no location search query")
		}
	}

	if len(s.availabilities) > 0 {
		results = s.availableUsers(found)
	} else {
		now := time.Now()
		// LRNT-253 Temporary fix, exclude from results tutor with no available slots from now
		for _, tutor := range found {
			hasAvailability := false
			availability := tutor.Tutoring.Availability
			if availability == nil {
				continue
			}

			slots := availability.Slots
			recurrent := availability.Recurrent
			if slots == nil && recurrent == nil {
				continue
			}

			if slots != nil {
				for _, slot := range slots {
					if slot.To.After(now) && slot.From.After(now) {
						hasAvailability = true
						break
					}
				}
			}

			// no need for date comparisons
			if recurrent != nil {
				hasAvailability = true
			}

			if hasAvailability {
				results = append(results, tutor)
			}
		}
	}

	return sortSearchResults(results), nil
}

func sortSearchResults(r []store.UserMgo) []store.UserMgo {
	sort.SliceStable(r, func(i, j int) bool {
		return r[i].Online > r[j].Online
	})
	//TODO: Remove duplicates
	return r
}

func (s *search) availableUsers(tutors []store.UserMgo) (available []store.UserMgo) {

	available = make([]store.UserMgo, 0)

	for _, tutor := range tutors {
		for _, av := range s.availabilities {
			if tutor.IsAvailable(av.From, av.To, false) {
				available = append(available, tutor)
			}
		}
	}

	return
}
