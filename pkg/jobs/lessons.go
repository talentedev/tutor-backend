package jobs

import (
	"time"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/services/delivery"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
)

var d *delivery.Delivery

func init() {
	d = delivery.New(config.GetConfig())
}

type TemplateNotificationPair struct {
	Duration time.Duration
	Template m.Tpl
}

type UpcomingLessonReminder struct {
	Pairs []TemplateNotificationPair
}

func (lr UpcomingLessonReminder) RemindUpcomingLesson() {
	now := time.Now().UTC().Truncate(time.Minute)
	for _, pair := range lr.Pairs {
		startTime := now.Add(pair.Duration)
		lessons := store.GetLessonsStore().GetLessonsAtTime(startTime)
		logger.Get().Debugf("lessons found: %+v", lessons)
		for _, lesson := range lessons {
			go emailLessonReminder(lesson, pair.Template)
		}
	}
}

func emailLessonReminder(lesson store.LessonMgo, tpl m.Tpl) {
	var roomURL string
	var err error

	participants := lesson.GetParticipants()
	room, err := services.VCRInstance().GetRoomForLesson(&lesson)
	if err != nil {
		logger.Get().Error(err.Error())
	}

	if room != nil {
		roomURL, err = core.AppURL("/room/%s", room.ID.Hex())
		if err != nil {
			logger.Get().Error(err.Error())
		}
	}

	for _, user := range participants {
		lessonDayTime := lesson.WhenFormattedWithTimezone(user.Timezone)

		logger.Get().Infof("sending reminder email to user, %s, %s", user.GetFirstName(), lessonDayTime)
		go d.Send(user, tpl, &m.P{
			"FIRST_NAME":    user.GetFirstName(),
			"CLASSROOM_URL": roomURL,
		})
	}
}

type DailyLessonReminder struct {
	NotifyTimes []time.Duration
}

func (lr DailyLessonReminder) DailyReminder() {
	now := time.Now().UTC().Truncate(time.Minute)

	for _, duration := range lr.NotifyTimes {
		endTime := now.Add(duration)
		lessons := store.GetLessonsStore().GetLessonsWithin24Hours(now, endTime)
		logger.Get().Infof("lessons found: %+v", lessons)
		for _, lesson := range lessons {
			go emailDailyReminder(lesson, now)
		}
	}
}

func emailDailyReminder(lesson store.LessonMgo, now time.Time) {
	participants := lesson.GetParticipants()

	for _, user := range participants {
		nowWithTimezone := now

		loc, err := time.LoadLocation(user.Timezone)
		if err == nil {
			nowWithTimezone = nowWithTimezone.In(loc)
		}
		// if time is 7am
		if nowWithTimezone.Hour() == 7 {
			var roomURL string
			var err error
			room, err := services.VCRInstance().GetRoomForLesson(&lesson)
			if err != nil {
				logger.Get().Error(err.Error())
			}

			if room != nil {
				roomURL, err = core.AppURL("/room/%s", room.ID.Hex())
				if err != nil {
					logger.Get().Error(err.Error())
				}
			}

			go d.Send(user, m.TPL_LESSON_REMINDER_7AM, &m.P{
				"FIRST_NAME":    user.GetFirstName(),
				"CLASSROOM_URL": roomURL,
			})
		}
	}
}

type UpcomingLessonReminderInTwoMinutes struct {
	NotifyTimes []time.Duration
}

func (lr UpcomingLessonReminderInTwoMinutes) RemindUpcomingLesson() {
	now := time.Now().UTC().Truncate(time.Minute)
	for _, duration := range lr.NotifyTimes {
		startTime := now.Add(duration)
		lessons := store.GetLessonsStore().GetLessonsAtTime(startTime)
		logger.Get().Infof("lessons found: %+v", lessons)
		for _, lesson := range lessons {
			go emailLessonReminderInTwoMinutes(lesson)
		}
	}
}

func emailLessonReminderInTwoMinutes(lesson store.LessonMgo) {
	tutor := lesson.GetTutor()
	var roomURL string
	var err error

	room, err := services.VCRInstance().GetRoomForLesson(&lesson)
	if err != nil {
		logger.Get().Error(err.Error())
	}

	if room != nil {
		roomURL, err = core.AppURL("/room/%s", room.ID.Hex())
		if err != nil {
			logger.Get().Error(err.Error())
		}
	}

	participants := lesson.GetParticipants()

	var studentName string
	for _, user := range participants {
		lessonDayTime := lesson.WhenFormattedWithTimezone(user.Timezone)
		if user.IsStudent() {
			studentName = user.GetFirstName()
			logger.Get().Infof("sending reminder email to student, %s, for %s", studentName, lessonDayTime)
			go d.Send(user, m.TPL_SCHEDULED_LESSON_IS_STARTING, &m.P{
				"FIRST_NAME":    user.GetFirstName(),
				"OTHER_NAME":    tutor.GetFirstName(),
				"CLASSROOM_URL": roomURL,
			})
		}
	}

	// send tutor email
	logger.Get().Infof("sending reminder email to tutor, %s, for %s", tutor.GetFirstName(), lesson.WhenFormattedWithTimezone(tutor.Timezone))

	go d.Send(tutor, m.TPL_SCHEDULED_LESSON_IS_STARTING, &m.P{
		"FIRST_NAME":    tutor.GetFirstName(),
		"OTHER_NAME":    studentName,
		"CLASSROOM_URL": roomURL,
	})
}

type WeeklyProfileReminder struct{}

func (lr WeeklyProfileReminder) WeeklyProfileReminder() {
	const threeWeeksAgo = 21
	tutors, count, _ := services.NewUsers().GetApprovedTutors()
	logger.Get().Infof("tutors found: %d", count)
	now := time.Now()
	for _, tutor := range tutors {
		if tutor.Tutoring.ProfileChecked == nil {
			tutor.SetProfileChecked(&now)
		}
		if utils.DateLessThanEqualDaysAgo(*tutor.Tutoring.ProfileChecked, threeWeeksAgo) {
			if !tutor.HasAvailability() || !tutor.HasSubjects() || !tutor.HasPhoto() {
				logger.Get().Debugf("would send reminder for tutor: %+v", tutor)
				go d.Send(tutor, m.TPL_INCOMPLETE_PROFILE, &m.P{
					"FIRST_NAME": tutor.GetFirstName(),
				})
			}
		}
	}
}
