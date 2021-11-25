package ics

import (
	"crypto/md5"
	"fmt"
	ical "github.com/arran4/golang-ical"
	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/store"
	"io"
	"strings"
	"time"
)

type lessons struct{}

func Lessons() *lessons {
	return &lessons{}
}

func (t *lessons) Serve(c *gin.Context, user *store.UserMgo, items []*store.LessonDto) (err error) {
	return t.Write(c.Writer, user, items)
}

func (t *lessons) Write(w io.Writer, user *store.UserMgo, items []*store.LessonDto) (err error) {
	cal := ical.NewCalendar()
	cal.SetMethod(ical.MethodRequest)

	for _, item := range items {
		lessonNotesURL, err := core.AppURL("/main/account/calendar/details/%s", item.ID.Hex())
		if err != nil {
			return err
		}
		start := item.StartsAt
		end := item.EndsAt
		summary := fmt.Sprintf("Learnt Session: %s", item.Subject.Name)
		desc := fmt.Sprintf("Lesson Details: %s", lessonNotesURL)

		toBeHashed := fmt.Sprintf("%s%s%s%s", start, end, summary, desc)
		id := fmt.Sprintf("%x", md5.Sum([]byte(toBeHashed)))
		event := cal.AddEvent(id)
		event.SetCreatedTime(time.Now())
		event.SetModifiedAt(item.CreatedAt)
		event.SetDtStampTime(time.Now())
		event.SetStartAt(start)
		event.SetEndAt(end)
		event.SetSummary(summary)
		event.SetLocation("")
		event.SetStatus(ical.ObjectStatusConfirmed)
		event.SetDescription(desc)
		event.SetTimeTransparency(ical.TransparencyOpaque)

		// TO-DO: may need to fork the lib as it doesn't support the whole RFC, e.g. X-WR-TIMEZONE, CALSCALE, etc
		// the organizer appears to be the first in accepted.
		if item.Accepted[0].Emails != nil {
			event.SetOrganizer(fmt.Sprintf("mailto:%s", item.Accepted[0].Emails[0].Email), ical.WithCN(fmt.Sprintf("%s", item.Accepted[0].Emails[0].Email)))
		}

		for _, att := range item.Accepted {
			if att.Emails != nil {
				// do we assume accepted?
				event.AddAttendee(fmt.Sprintf("%s", att.Emails[0].Email), ical.CalendarUserTypeIndividual, ical.ParticipationRoleReqParticipant, ical.ParticipationStatusAccepted, ical.WithCN(fmt.Sprintf("%s", att.Emails[0].Email)))
			}
		}

	}
	r := strings.NewReader(cal.Serialize())
	_, err = r.WriteTo(w)
	if err != nil {
		return err
	}

	return nil
}
