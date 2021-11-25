package services

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"time"

	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/utils/timeline"

	"github.com/olebedev/emitter"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/notifications"
	"gitlab.com/learnt/api/pkg/services/delivery"
	"gitlab.com/learnt/api/pkg/services/models"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/ws"
	"gopkg.in/mgo.v2/bson"
)

// LessonErrType is an int to describe types of possible errors
type LessonErrType uint

const (
	daysPerWeek                  = 7
	errInvalidUser LessonErrType = iota
	errInvalidRole
	errInvalidSubject
	errInvalidTime
	errDatabase
	errInvalidProposal
	errInvalidMeetingPlace
)

// LessonErr is the HTTP response for a lesson error
type LessonErr struct {
	Type    LessonErrType `json:"type"`
	Message string        `json:"message"`
}

// Error fulfills the error interface to make this returnable as an error
func (e *LessonErr) Error() string {
	return e.Message
}

func newLessonErr(t LessonErrType, m string) *LessonErr {
	return &LessonErr{Type: t, Message: m}
}

type Lessons struct {
	store       *store.LessonsStore
	nextTimeout time.Duration
	nextTick    chan int
}

var lessonInst *Lessons

// GetLessons gets an object used to interact with the Lessons db
func GetLessons() *Lessons {
	if lessonInst != nil {
		return lessonInst
	}

	lessonInst = &Lessons{
		store:       store.GetLessonsStore(),
		nextTimeout: time.Second,
		nextTick:    make(chan int),
	}

	return lessonInst
}

func (l *Lessons) DetectNextTimeout(yes ...bool) {
	defer logger.Get().Info("detectnexttimeout done")
	var lesson *store.LessonMgo

	query := store.GetCollection("lessons").Find(bson.M{"starts_at.date": bson.M{"$gt": time.Now()}})
	_ = query.Sort("starts_at.date").One(&lesson)

	if lesson == nil {
		l.nextTimeout = time.Minute
		return
	}

	l.nextTimeout = lesson.StartsAt.Sub(time.Now())

	logger.Get().Info("Lessons Loop: timeout updated to:", l.nextTimeout)

	if len(yes) > 0 && yes[0] {
		select {
		case l.nextTick <- 1:
		case <-time.After(time.Second):
		}
	}
}

// LessonMarker used for updating Lessons
type LessonMarker struct {
	Lesson *store.LessonMgo
	key    string
	at     time.Time
}

// At gets the marker time
func (lm *LessonMarker) At() time.Time {
	return lm.at
}

// Key gets the marker key
func (lm *LessonMarker) Key() string {
	return lm.key
}

func (l *Lessons) provideUpdateMarkers(from, to time.Time) (markers []timeline.Marker) {
	for _, lesson := range l.store.GetLessons() {
		var before int

		if !lesson.Notifications.OneDayBefore {
			at := lesson.StartsAt.Add(-time.Hour * 24)
			if at.After(time.Now()) {
				before++
				markers = append(markers, &LessonMarker{
					Lesson: &lesson,
					key:    store.LessonNotificationOneDay,
					at:     at,
				})
			}
		}

		if !lesson.Notifications.ThirtiethMinutesBefore {
			at := lesson.StartsAt.Add(-time.Minute * 30)
			if at.After(time.Now()) {
				before++
				markers = append(markers, &LessonMarker{
					Lesson: &lesson,
					key:    store.LessonNotificationThirtyMinutes,
					at:     at,
				})
			}
		}

		if !lesson.Notifications.TenMinutesBefore {
			at := lesson.StartsAt.Add(-time.Minute * 10)
			if at.After(time.Now()) {
				before++
				markers = append(markers, &LessonMarker{
					Lesson: &lesson,
					key:    store.LessonNotificationTenMinutes,
					at:     at,
				})
			}
		}

		if before == 0 && lesson.StartsAt.After(time.Now()) {
			markers = append(markers, &LessonMarker{
				Lesson: &lesson,
				key:    store.LessonNotificationBefore,
				at:     time.Now(),
			})
		}

		markers = append(markers, &LessonMarker{
			Lesson: &lesson,
			key:    "start",
			at:     lesson.StartsAt.Add(0),
		})
	}

	return markers
}

func (l *Lessons) GetCurrentRunningLessons(offset int, limit int) (*store.PaginatedLessons, error) {
	return store.GetLessonsStore().GetCurrentRunningLessons(offset, limit)
}

func (l *Lessons) GetDefaultLessons() []store.LessonMgo {
	return store.GetLessonsStore().GetDefaultLessons()
}

func (l *Lessons) GetInstantLessons() []store.LessonMgo {
	return store.GetLessonsStore().GetInstantLessons()
}

func (l *Lessons) Start(ctx context.Context) {
	u := timeline.NewUpdater(ctx, l.provideUpdateMarkers)

	u.SetOnMarker(func(m timeline.Marker) {
		v := reflect.ValueOf(m)
		marker := v.Interface().(*LessonMarker)

		switch m.Key() {
		case store.LessonNotificationOneDay:
		case store.LessonNotificationThirtyMinutes:
		case store.LessonNotificationTenMinutes:
		case store.LessonNotificationBefore:
			l.NotifyBeforeStarts(marker.Lesson, marker.Lesson.StartsAt.Sub(m.At()), m.Key())
		case "start":
			lesson := marker.Lesson

			// cancel if not confirmed and still pending
			if lesson.State == store.LessonBooked {
				if err := lesson.SetState(store.LessonCancelled, nil); err != nil {
					logger.Get().Errorf("failed to set lesson state to CANCELLED: %v", err)
				}
				return
			}

			if _, err := VCRInstance().GetRoomForLesson(marker.Lesson); err != nil {
				logger.Get().Errorf("failed to get lesson room: %v", err)
				return
			}

			if err := marker.Lesson.SetState(store.LessonProgress, nil); err != nil {
				logger.Get().Errorf("failed to set lesson state to PROGRESS: %v", err)
				return
			}

			title := "Lesson just started"
			message := fmt.Sprintf("Lesson with tutor %s and students %s just started", marker.Lesson.GetTutor().Name(), marker.Lesson.StudentsNames(true))
			l.NotifyAll(marker.Lesson, notifications.LessonStarted, title, message)
		}
	})

	utils.Bus().On(utils.EvLessonCreated, func(e *emitter.Event) {
		u.Sync()
	})

	go func() { <-u.Run() }()
}

// CreateLessonRequest is the HTTP body used for creating a new lesson
type CreateLessonRequest struct {
	Tutor          bson.ObjectId `json:"tutor" binding:"required"`
	Student        bson.ObjectId `json:"student" binding:"required"`
	Subject        bson.ObjectId `json:"subject" binding:"required"`
	When           time.Time     `json:"when" binding:"required"`
	Duration       string        `json:"duration" binding:"required"`
	Meet           store.Meet    `json:"meet" binding:"required"`
	Location       string        `json:"location"`
	RecurrentCount int           `json:"recurrent_count"`
	Recurrent      bool          `json:"recurrent"`
	Instant        bool          `json:"instant"`
}

/*type authorization struct {
	student  *store.UserMgo
	tutor    *store.UserMgo
	duration float64
	lessonID string
	chargeID string
}*/

func (l *Lessons) AuthorizeCharges(lesson *store.LessonMgo) {
	p := GetPayments()
	duration := math.Ceil(lesson.Duration().Minutes())

	tutor, ok := NewUsers().ByID(lesson.Tutor)
	if !ok {
		logger.Get().Error("couldn't get tutor from database")
	}

	// all students are charged for the amount
	for _, studentID := range lesson.Students {
		student, ok := NewUsers().ByID(studentID)
		if !ok {
			logger.Get().Error("couldn't get student from database")
			continue
		}

		if !student.IsTestStudent() {
			charge, err := p.ChargeForLesson(student, tutor, duration, lesson.StartsAtDateTimeFormatted(), lesson.ID.Hex(), false, lesson.Rate)
			if err != nil {
				logger.Get().Errorf("couldn't charge student %s on lesson %v: %v\n", student.Name(), lesson.ID.Hex(), err)
				return
			}
			l.SaveCharges(lesson.ID, charge)
		}
	}
}

// Per-Student AuthorizeCharge.
func (l *Lessons) AuthorizeCharge(student *store.UserMgo, lesson *store.LessonMgo) {
	p := GetPayments()
	duration := math.Ceil(lesson.Duration().Minutes())

	tutor, ok := NewUsers().ByID(lesson.Tutor)
	if !ok {
		logger.Get().Error("couldn't get tutor from database")
	}

	if !student.IsTestStudent() {
		charge, err := p.ChargeForLesson(student, tutor, duration, lesson.StartsAtDateTimeFormatted(), lesson.ID.Hex(), false, lesson.Rate)
		if err != nil {
			logger.Get().Errorf("couldn't charge student %s on lesson %v: %v\n", student.Name(), lesson.ID.Hex(), err)
		}
		l.SaveCharges(lesson.ID, charge)
	}
}

func (l *Lessons) CreateInstantSession(student *store.UserMgo, tutor *store.UserMgo, subject *store.Subject) (err error) {
	lesson := &store.LessonMgo{
		ID:        bson.NewObjectId(),
		Tutor:     tutor.ID,
		Students:  []bson.ObjectId{student.ID},
		StartsAt:  time.Now(),
		EndsAt:    time.Now().Add(time.Hour * 1),
		Rate:      tutor.Tutoring.Rate,
		Meet:      store.MeetOnline,
		State:     store.LessonConfirmed,
		Subject:   subject.ID,
		CreatedAt: time.Now(),
		Accepted:  []bson.ObjectId{student.ID, tutor.ID},
		Type:      store.LessonInstant,
	}

	if errIns := store.GetCollection("lessons").Insert(lesson); errIns != nil {
		return newLessonErr(errDatabase, "couldn't insert the lesson")
	}

	room, err := VCRInstance().GetRoomForLesson(lesson)

	if err != nil {
		go store.GetCollection("lessons").RemoveId(lesson.ID)
		return errors.Wrap(err, "Failed to create room for instant session lesson")
	}

	for _, participant := range lesson.GetParticipants() {
		if uc := ws.GetEngine().Hub.User(participant.ID); uc != nil {
			_ = uc.Send(ws.Event{
				Type: "instant.start",
				Data: ws.EventData{
					"room": room.ID.Hex(),
				},
			})
		}
	}

	return nil
}

func (l *Lessons) Create(user *store.UserMgo, request *CreateLessonRequest) (store.LessonMgo, error) {
	users := NewUsers()

	subject, exist := store.GetSubject(request.Subject)
	if !exist || !subject.ID.Valid() {
		return store.LessonMgo{}, newLessonErr(errInvalidSubject, "subject does not exist")
	}

	tutor, tutorExist := users.ByID(request.Tutor)
	if !tutorExist {
		return store.LessonMgo{}, newLessonErr(errInvalidUser, "tutor does not exist")
	}

	student, studentExist := users.ByID(request.Student)
	if !studentExist {
		return store.LessonMgo{}, newLessonErr(errInvalidUser, "student does not exist")
	}

	if !tutor.IsTutor() {
		return store.LessonMgo{}, newLessonErr(errInvalidRole, "tutor is pending")
	}

	if tutor.Tutoring.Meet != store.MeetBoth && tutor.Tutoring.Meet != request.Meet {
		return store.LessonMgo{}, newLessonErr(errInvalidMeetingPlace, fmt.Sprintf("tutor is not available to %s", request.Meet.String()))
	}

	lessonDate := request.When.UTC()

	if lessonDate.Before(time.Now()) {
		return store.LessonMgo{}, newLessonErr(errInvalidTime, "lesson can't be in the past")
	}

	advanceDurationString := config.GetConfig().GetString("Lessons.advance_duration")

	advanceDuration, err := time.ParseDuration(advanceDurationString)
	if err != nil {
		advanceDuration = time.Hour
	}

	minTimeLesson := time.Now().Add(advanceDuration)
	if lessonDate.Before(minTimeLesson) {
		message := fmt.Sprintf("lesson can't be created with %s in advance", advanceDurationString)
		return store.LessonMgo{}, newLessonErr(errInvalidTime, message)
	}

	requestedDuration, err := time.ParseDuration(request.Duration)
	if err != nil {
		return store.LessonMgo{}, newLessonErr(errInvalidTime, "invalid duration")
	}

	if !tutor.IsAvailable(lessonDate, lessonDate.Add(requestedDuration), request.Recurrent) {
		return store.LessonMgo{}, newLessonErr(errInvalidTime, "tutor doesn't have availability set")
	}

	if !tutor.IsFree(lessonDate, requestedDuration) {
		return store.LessonMgo{}, newLessonErr(errInvalidTime, "tutor is not free")
	}

	if !student.IsFree(lessonDate, requestedDuration) {
		return store.LessonMgo{}, newLessonErr(errInvalidTime, "student is not free")
	}

	state := store.LessonBooked
	notificationType := notifications.LessonBooked
	acceptedUsers := []bson.ObjectId{user.ID}

	if tutor.Tutoring.InstantBooking {
		state = store.LessonConfirmed
		notificationType = notifications.LessonAccepted
		acceptedUsers = append(acceptedUsers, tutor.ID)
	}

	startsAtDate := lessonDate
	endsAtDate := lessonDate.Add(requestedDuration)

	baseLesson := store.LessonMgo{
		Tutor:         tutor.ID,
		Students:      []bson.ObjectId{student.ID},
		StartsAt:      startsAtDate,
		EndsAt:        endsAtDate,
		Rate:          tutor.Tutoring.Rate,
		Meet:          request.Meet,
		Location:      request.Location,
		State:         state,
		Subject:       subject.ID,
		StateTimeline: []store.LessonStateData{},
		CreatedAt:     time.Now(),
		Accepted:      acceptedUsers,
		Recurrent:     request.Recurrent,
	}

	if request.RecurrentCount == 0 {
		request.RecurrentCount = 1
	}

	recurrentID := bson.NewObjectId()
	if request.RecurrentCount > 1 {
		baseLesson.RecurrentID = &recurrentID
	}

	insertedLessons := make([]store.LessonMgo, 0)
	for i := 1; i <= request.RecurrentCount; i++ {
		insertLesson := baseLesson
		insertLesson.ID = bson.NewObjectId()

		if errIns := store.GetCollection("lessons").Insert(insertLesson); errIns != nil {
			return store.LessonMgo{}, newLessonErr(errDatabase, fmt.Sprintf("couldn't insert the lesson -- %s", errIns.Error()))
		}

		insertedLessons = append(insertedLessons, insertLesson)
		baseLesson.StartsAt = baseLesson.StartsAt.AddDate(0, 0, 7)
		baseLesson.EndsAt = baseLesson.EndsAt.AddDate(0, 0, 7)
	}

	notifyTitle := "New lesson created"
	if request.RecurrentCount > 1 {
		notifyTitle = "New recurring lesson created"
	}

	notifyAction := fmt.Sprintf("/main/account/calendar/details/%s", insertedLessons[0].ID.Hex())
	for _, userID := range []bson.ObjectId{tutor.ID, user.ID} {
		notifyMsg := fmt.Sprintf("New lesson created with %s", tutor.Name())
		if userID.Hex() == insertedLessons[0].Tutor.Hex() {
			notifyMsg = fmt.Sprintf("New lesson created with %s ", insertedLessons[0].StudentsNames(false))
		}

		notifications.Notify(&notifications.NotifyRequest{
			User:    userID,
			Type:    notificationType,
			Title:   notifyTitle,
			Message: notifyMsg,
			Action:  &notifyAction,
			Data:    map[string]interface{}{"lesson": insertedLessons[0]},
		})
	}
	d := delivery.New(config.GetConfig())
	if baseLesson.Tutor.Hex() == user.ID.Hex() {
		// if the user who booked is the tutor
		for _, studentID := range baseLesson.Students {
			if student, exist := NewUsers().ByID(studentID); !exist {
				notifyActionURL, err := core.AppURL(notifyAction)
				if err != nil {
					return store.LessonMgo{}, err
				}

				go d.Send(student, m.TPL_TUTOR_PROPOSED_LESSON, &m.P{
					"TUTOR_NAME":   tutor.Name(),
					"STUDENT_NAME": baseLesson.StudentsNames(true),
					"DAY_TIME":     baseLesson.WhenFormatted(),
					"CHAT_URL":     notifyActionURL,
				})
			}
		}
	} else {
		notifyActionURL, err := core.AppURL(notifyAction)
		if err != nil {
			return store.LessonMgo{}, err
		}

		// if the user who booked is the student
		d.Send(tutor, m.TPL_STUDENT_PROPOSED_LESSON, &m.P{
			"TUTOR_NAME":   tutor.Name(),
			"STUDENT_NAME": baseLesson.StudentsNames(false),
			"DAY_TIME":     baseLesson.WhenFormattedWithTimezone(tutor.Timezone),
			"CALENDAR_URL": notifyActionURL,
		})

		for _, studentID := range baseLesson.Students {
			if studentID.Hex() == user.ID.Hex() {
				continue
			}

			notifyActionURL, err := core.AppURL(notifyAction)
			if err != nil {
				return store.LessonMgo{}, err
			}

			if student, exist := NewUsers().ByID(studentID); exist {
				_ = d.Send(student, m.TPL_TUTOR_PROPOSED_LESSON, &m.P{
					"TUTOR_NAME":   tutor.Name(),
					"STUDENT_NAME": baseLesson.StudentsNames(true),
					"DAY_TIME":     baseLesson.WhenFormattedWithTimezone(student.Timezone),
					"CHAT_URL":     notifyActionURL,
				})
			}
		}
	}

	done := utils.Bus().Emit(utils.EvLessonCreated, insertedLessons[0], user)
	select {
	case <-done:
	case <-time.After(time.Second):
		close(done)
	}

	l.DetectNextTimeout(true)

	return insertedLessons[0], nil

}

func (l *Lessons) ProposeChange(lesson *store.LessonMgo, user *store.UserMgo, change store.LessonChangeProposal) error {
	if !change.User.Valid() {
		change.User = user.ID
	}

	intervalChanged := !change.StartsAt.IsZero() && !change.EndsAt.IsZero()

	if intervalChanged && (change.StartsAt.Before(time.Now()) || change.EndsAt.Before(time.Now())) {
		return newLessonErr(errInvalidTime, "date for start or end can't be in the past")
	}

	if change.CreatedAt.IsZero() {
		change.CreatedAt = time.Now()
	}

	if !change.Subject.Valid() {
		change.Subject = lesson.Subject
	}

	if change.Meet != store.MeetOnline && change.Meet != store.MeetInPerson {
		change.Meet = lesson.Meet
	}

	if change.Location == "" {
		change.Location = lesson.Location
	}

	tutor, ok := NewUsers().ByID(lesson.Tutor)
	if !ok {
		return newLessonErr(errInvalidUser, "couldn't get tutor by id")
	}

	if err := lesson.ProposeChange(change); err != nil {
		err = errors.Wrap(err, "couldn't propose lesson changes")
		return newLessonErr(errInvalidProposal, err.Error())
	}

	title := fmt.Sprintf("%s proposed changes for a lesson", user.Name())
	message := fmt.Sprintf("%s proposed some changes for the lesson %s", user.Name(), lesson.WhenFormatted())

	_ = l.NotifyExcept(lesson, user, notifications.LessonChangeRequest, title, message)

	var mailTemplate m.Tpl

	mailData := m.P{
		"TUTOR_NAME": tutor.Name(),
		"USER_NAME":  user.Name(),
		"DAY_TIME":   lesson.WhenFormattedWithTimezone(tutor.Timezone),
	}

	var student *store.UserMgo
	for _, studentID := range lesson.Students {
		student_, exists := NewUsers().ByID(studentID)
		if exists {
			student = student_
			mailData["STUDENT_NAME"] = student.Name()
			break
		}
	}

	if student == nil {
		logger.Get().Error("Student not found")
		return errors.New("Student not found in lesson")
	}

	d := delivery.New(config.GetConfig())
	if user.ID.Hex() == tutor.ID.Hex() {
		mailTemplate = m.TPL_TUTOR_PROPOSED_LESSON_CHANGE
		go d.Send(student, mailTemplate, &mailData)
	}
	if user.ID.Hex() == student.ID.Hex() {
		mailTemplate = m.TPL_STUDENT_PROPOSED_LESSON_CHANGE
		go d.Send(tutor, mailTemplate, &mailData)
	}

	return nil
}

func (l *Lessons) Accept(lesson *store.LessonMgo, user *store.UserMgo) (err error) {
	everyone, err := lesson.AcceptBy(user)
	if err != nil {
		return errors.Wrap(err, "Failed to accept the lesson by the user")
	}

	if everyone {
		_ = lesson.SetState(store.LessonConfirmed, nil)
	}

	return
}

func (l *Lessons) Reject(user *store.UserMgo, lesson *store.LessonMgo, reason string) (err error) {
	if err := lesson.Reject(user, reason); err != nil {
		return errors.Wrap(err, "Failed to reject the lesson")
	}

	message := fmt.Sprintf("Lesson cannceled by %s with reason: %s", user.Name(), reason)

	var notifyKind notifications.NotifyKind

	role, found := user.RoleForLesson(lesson)

	if found {
		switch role {
		case store.RoleStudent:
			notifyKind = notifications.LessonStudentCancelled
		case store.RoleAdmin:
			notifyKind = notifications.LessonTutorCancelled
		}
	}

	// Notify tutor
	notifications.Notify(&notifications.NotifyRequest{
		User:    lesson.Tutor,
		Type:    notifyKind,
		Title:   "Lesson cancelled",
		Message: message,
		Data: map[string]interface{}{
			"lesson": lesson,
		},
	})

	// Notify students
	for _, student := range lesson.Students {
		notifications.Notify(&notifications.NotifyRequest{
			User:    student,
			Type:    notifyKind,
			Title:   "Lesson cancelled",
			Message: message,
			Data: map[string]interface{}{
				"lesson": lesson,
			},
		})
	}

	return nil
}

func (l *Lessons) NotifyAll(lesson *store.LessonMgo, notifyKind notifications.NotifyKind, title, message string) {
	users := make([]*store.UserMgo, 0)

	tutor, _ := NewUsers().ByID(lesson.Tutor)
	users = append(users, tutor)
	users = append(users, NewUsers().ByIDs(lesson.Students)...)

	for _, user := range users {
		// Notify everybody
		notifications.Notify(&notifications.NotifyRequest{
			User:    user.ID,
			Type:    notifyKind,
			Title:   title,
			Message: message,
			Data: map[string]interface{}{
				"lesson": lesson,
			},
		})
	}
}

func (l *Lessons) NotifyExcept(lesson *store.LessonMgo, user *store.UserMgo, notifyKind notifications.NotifyKind, title, message string) error {
	if lesson.Tutor.Hex() != user.ID.Hex() {
		notifications.Notify(&notifications.NotifyRequest{
			User:    lesson.Tutor,
			Type:    notifyKind,
			Title:   title,
			Message: message,
			Data:    map[string]interface{}{"lesson": lesson},
		})
	}

	for _, student := range lesson.Students {
		if student.Hex() == user.ID.Hex() {
			continue
		}

		action := "/Lessons/" + lesson.ID.Hex()

		notifications.Notify(&notifications.NotifyRequest{
			User:    student,
			Type:    notifyKind,
			Title:   title,
			Message: message,
			Action:  &action,
			Data:    map[string]interface{}{"lesson": lesson},
		})
	}

	return nil
}

func (l *Lessons) NotifyBeforeStarts(lesson *store.LessonMgo, duration time.Duration, key string) error {
	if !lesson.IsConfirmed() || lesson.IsNotified() {
		return nil
	}

	switch key {
	case store.LessonNotificationOneDay:
		if lesson.Notifications.OneDayBefore {
			return nil
		}

		lesson.Notifications.OneDayBefore = true
	case store.LessonNotificationThirtyMinutes:
		if lesson.Notifications.ThirtiethMinutesBefore {
			return nil
		}

		lesson.Notifications.ThirtiethMinutesBefore = true
	case store.LessonNotificationTenMinutes:
		if lesson.Notifications.TenMinutesBefore {
			return nil
		}
	case store.LessonNotificationBefore:
	default:
		panic("Invalid notification")
	}

	if err := lesson.SetIsNotified(key); err != nil {
		panic(err)
	}
	d := delivery.New(config.GetConfig())
	for _, participant := range lesson.GetParticipants() {
		title := fmt.Sprintf("%s lesson will start %s", lesson.FetchSubjectName(), lesson.WhenFormatted())
		message := fmt.Sprintf("%s lesson with tutor %s and students %s will start %s", lesson.FetchSubjectName(), participant.Name(),
			lesson.StudentsNames(true), lesson.WhenFormatted())

		dto, _ := lesson.DTO()

		// Notify tutor
		response := <-notifications.Notify(&notifications.NotifyRequest{
			User:    participant.ID,
			Type:    notifications.LessonNotifyBefore,
			Title:   title,
			Message: message,
			Data: map[string]interface{}{
				"lesson": dto,
			},
		})

		if !response.Succeed {
			room, err := VCRInstance().GetRoomForLesson(lesson)
			if err != nil {
				logger.Get().Errorf("failed to get room for lesson: %v", err)
				return nil
			}

			roomURL, err := core.AppURL("/room/%s", room.ID.Hex())
			if err != nil {
				return err
			}

			_ = d.Send(
				participant,
				m.TPL_LESSON_STARTING,
				&m.P{
					"SUBJECT":          title,
					"LESSON_STARTS_AT": lesson.WhenFormatted(),
					"TUTOR_NAME":       lesson.GetTutor().Name(),
					"SUBJECT_NAME":     lesson.FetchSubjectName(),
					"STUDENT_NAMES":    lesson.StudentsNames(false),
					"LESSON_LINK":      roomURL,
				},
			)
		}
	}

	// FIXME
	//lesson.SaveRoom()

	return nil
}

// TODO: Same as (*Lessons).NotifyAll()
func (l *Lessons) Notify(user *store.UserMgo, lesson *store.LessonMgo, notifyKind notifications.NotifyKind, title, message string) {
	dto, err := lesson.DTO()
	if err != nil {
		logger.Get().Errorf("error getting lesson dto: %v", err)
		return
	}

	// Notify tutor
	notifications.Notify(&notifications.NotifyRequest{
		User:    user.ID,
		Type:    notifyKind,
		Title:   title,
		Message: message,
		Data: map[string]interface{}{
			"lesson": dto,
		},
	})

	// Notify students
	for _, student := range lesson.Students {
		notifications.Notify(&notifications.NotifyRequest{
			User:    student,
			Type:    notifyKind,
			Title:   title,
			Message: message,
			Data: map[string]interface{}{
				"lesson": dto,
			},
		})
	}
}

func (l *Lessons) CompleteUnAuthorized(lesson *store.LessonMgo) error {
	if err := lesson.SetState(store.LessonCompleted, nil); err != nil {
		return fmt.Errorf("couldn't set complete state: %s", err)
	}

	now := time.Now()

	return lesson.SetEndsAt(now)
}

func (l *Lessons) Complete(lesson *store.LessonMgo) error {
	if err := lesson.SetState(store.LessonCompleted, nil); err != nil {
		return fmt.Errorf("couldn't set complete state: %s", err)
	}

	now := time.Now()
	if lesson.IsInstantSession() {
		if err := lesson.SetEndsAt(now); err != nil {
			return fmt.Errorf("couldn't set complete state: %s", err)
		}

		lesson.EndsAt = now
	}

	referLinks, err := GetRefers().NeedPayment()
	if err != nil {
		return fmt.Errorf("couldn't get refer links: %s", err)
	}

	if len(referLinks) == 0 {
		return nil
	}

	transactionDetails := fmt.Sprintf("Transaction for lesson with ID %s", lesson.ID.Hex())

	tutor, ok := NewUsers().ByID(lesson.Tutor)
	if !ok {
		return fmt.Errorf("couldn't get lesson's tutor: %s", err)
	}

	lessonIntf := lessonInterface{
		ID:       lesson.ID,
		Tutor:    tutor,
		StartsAt: lesson.StartsAt,
	}

	amount := float64(tutor.Tutoring.Rate/60) * lesson.Duration().Minutes()

	for _, link := range referLinks {
		// check for students' link completion
		for _, studentID := range lesson.Students {
			// currently a single student, but we're iterating for the sake of a slice
			if link.Referral.Hex() == studentID.Hex() {
				student, ok := NewUsers().ByID(studentID)
				if !ok {
					continue
				}

				lessonIntf.Student = student
				switch link.Bond {
				case store.AffiliateToStudentBond, store.AffiliateToTutorBond:
					if err := completeLinkAndPayAffiliate(link, amount, transactionDetails, lessonIntf); err != nil {
						logger.Get().Errorf("couldn't complete refer link & pay student: %v", err)
					}
				default:
					if err := completeLinkAndPay(link, amount, transactionDetails, lessonIntf); err != nil {
						logger.Get().Errorf("couldn't complete refer link & pay student: %v", err)
					}
				}
			}
		}

		// check for tutor's link completion
		if link.Referral.Hex() == lesson.Tutor.Hex() {
			student, ok := NewUsers().ByID(lesson.Students[0])
			if !ok {
				return fmt.Errorf("couldn't get lesson's student")
			}

			lessonIntf.Student = student
			switch link.Bond {
			case store.AffiliateToStudentBond, store.AffiliateToTutorBond:
				if err := completeLinkAndPayAffiliate(link, amount, transactionDetails, lessonIntf); err != nil {
					logger.Get().Errorf("couldn't complete refer link & pay affiliate: %v", err)
				}
			default:
				if err := completeLinkAndPay(link, amount, transactionDetails, lessonIntf); err != nil {
					logger.Get().Errorf("couldn't complete refer link & pay tutor: %v", err)
				}
			}
		}
	}

	logger.Get().Debugf("Authorizing charges for lesson %v", lesson.ID)
	l.AuthorizeCharges(lesson)

	if err := checkForReview(lesson); err != nil {
		// todo: handle error
	}

	return nil
}

type lessonInterface struct {
	ID       bson.ObjectId
	Tutor    *store.UserMgo
	Student  *store.UserMgo
	StartsAt time.Time
}

func completeLinkAndPay(link *store.ReferLink, amount float64, details string, lesson lessonInterface) error {
	if link.Step == store.CompletedStep {
		return nil
	}

	referrerCredit := link.Amount

	referrer, ok := NewUsers().ByID(*link.Referrer)
	if !ok {
		return fmt.Errorf("referrer from refer link does not exist")
	}

	referral, ok := NewUsers().ByID(*link.Referral)
	if !ok {
		return fmt.Errorf("referral from refer link does not exist")
	}

	var isReferrerStudent bool

	ls := store.GetLessonsStore()
	lessons, err := ls.GetAllUserLessons(referral)
	if err != nil {
		return err
	}

	if referrer.IsStudentStrict() {
		isReferrerStudent = true
	}

	p := GetPayments()

	if isReferrerStudent {
		if len(lessons) == 0 || len(lessons) > 1 {
			return nil
		}

		if lessons[0].EndsAt.IsZero() {
			return fmt.Errorf("lesson hasn't ended")
		}

		reason := "credit"
		amount := int64(referrerCredit) * 100
		var creditReferrerParams, creditReferralParams CreditParams
		creditReferrerParams.Notes = fmt.Sprintf("Add %d credits to %s", reason, referrerCredit, referrer.Name())
		creditReferrerParams.Reason = reason
		creditReferrerParams.Amount = amount
		if err := p.AddCredits(referrer, creditReferrerParams); err != nil {
			return err
		}

		if _, err := GetTransactions().New(&store.TransactionMgo{
			User:    *link.Referrer,
			Amount:  referrerCredit,
			Lesson:  &lesson.ID,
			Details: details,
		}); err != nil {
			return fmt.Errorf("couldn't create transaction for referrer: %s", err)
		}

		creditReferralParams.Notes = fmt.Sprintf("Add %d credits to %s", reason, referrerCredit, referral.Name())
		creditReferralParams.Reason = reason
		creditReferralParams.Amount = amount
		if err := p.AddCredits(referral, creditReferralParams); err != nil {
			return err
		}

		if _, err := GetTransactions().New(&store.TransactionMgo{
			User:    *link.Referral,
			Amount:  referrerCredit,
			Lesson:  &lesson.ID,
			Details: details,
		}); err != nil {
			return fmt.Errorf("couldn't create transaction for referral: %s", err)
		}

		return link.Complete()
	}
	// referree had to be tutor
	// get total number of hours
	var total float64
	for _, l := range lessons {
		total += l.Duration().Hours()
	}

	if total >= 10 {
		if err := p.CreditForReferral(referrer, referrerCredit); err != nil {
			return fmt.Errorf("couldn't add balance to referrer: %s", err)
		}

		if _, err := GetTransactions().New(&store.TransactionMgo{
			User:    *link.Referrer,
			Amount:  referrerCredit,
			Lesson:  &lesson.ID,
			Details: details,
		}); err != nil {
			return fmt.Errorf("couldn't create transaction for referrer: %s", err)
		}

		if err := p.CreditForReferree(referral, 10); err != nil {
			return fmt.Errorf("couldn't add balance to referrer: %s", err)
		}

		if _, err := GetTransactions().New(&store.TransactionMgo{
			User:    *link.Referral,
			Amount:  10, // $10
			Lesson:  &lesson.ID,
			Details: details,
		}); err != nil {
			return fmt.Errorf("couldn't create transaction for referral: %s", err)
		}

		if err = link.Complete(); err != nil {
			return fmt.Errorf("couldn't complete refer link: %s", err)
		}
	}

	return nil
}

func completeLinkAndPayAffiliate(link *store.ReferLink, amount float64, details string, lesson lessonInterface) error {
	referCredit := (15.0 / 100.0) * amount

	referrer, ok := NewUsers().ByID(*link.Referrer)
	if !ok {
		return fmt.Errorf("referrer from refer link does not exist")
	}

	referral, ok := NewUsers().ByID(*link.Referral)
	if !ok {
		return fmt.Errorf("referral from refer link does not exist")
	}
	d := delivery.New(config.GetConfig())

	if referrer.HasBank() {
		_ = d.Send(referrer, m.TPL_AFFILIATE_PAYMENT_PENDING, &m.P{
			"FIRST_NAME":     referrer.Profile.FirstName,
			"STUDENT_NAME":   lesson.Student.Name(),
			"TUTOR_NAME":     lesson.Tutor.Name(),
			"LESSON_DAY":     utils.FormatTime(lesson.StartsAt),
			"PAYMENT_AMOUNT": fmt.Sprintf("$%.2f", referCredit),
			"LESSON_HISTORY": core.APIURL("/account/Lessons"),
		})
	} else {
		_ = d.Send(referrer, m.TPL_AFFILIATE_PAYMENT_NO_BANK_ACCOUNT, &m.P{
			"FIRST_NAME":        referrer.Profile.FirstName,
			"REFERRAL_NAME":     referral.Name(),
			"PAYMENT_AMOUNT":    fmt.Sprintf("$%.2f", referCredit),
			"BANK_ACCOUNT_LINK": core.APIURL("/account/payout"),
		})
	}

	p := GetPayments()
	if err := p.CreditForReferral(referrer, referCredit); err != nil {
		return fmt.Errorf("couldn't add balance to referrer: %s", err)
	}

	t := &store.TransactionMgo{
		User:    *link.Referrer,
		Amount:  referCredit,
		Lesson:  &lesson.ID,
		Details: details,
	}

	if _, err := GetTransactions().New(t); err != nil {
		return fmt.Errorf("couldn't create transaction for referrer: %s", err)
	}

	if link.Affiliate {
		if err := link.SetAmount(link.Amount + referCredit); err != nil {
			return fmt.Errorf("couldn't update refer link amount: %s", err)
		}
	}

	return link.Complete()
}

func checkForReview(lesson *store.LessonMgo) error {
	students, err := lesson.StudentsDto()
	if err != nil {
		return fmt.Errorf("couldn't get students dto: %s", err)
	}

	tutor := lesson.TutorDto()
	if tutor == nil {
		return fmt.Errorf("couldn't get tutor dto: %s", err)
	}

	var student store.PublicUserDto
	for _, s := range students {
		student = s
		if _, ok := tutor.GetReviewFrom(&student); ok {
			// student already reviewed the tutor
			continue
		}

		notifications.Notify(&notifications.NotifyRequest{
			User:    s.ID,
			Type:    notifications.LessonCompleteReview,
			Title:   "Leave a review",
			Message: fmt.Sprintf("What do you think about %s %s?", tutor.Profile.FirstName, tutor.Profile.LastName),
			Data:    map[string]interface{}{"user": tutor},
		})
	}

	return nil
}

func (l *Lessons) GetCompletedLessonsForUser(id bson.ObjectId, from, to time.Time) ([]*store.LessonDto, error) {
	lessons := make([]*store.LessonDto, 0)
	if err := store.GetCollection("lessons").Pipe([]bson.M{
		{
			"$match": bson.M{
				"state": store.LessonCompleted,
				"$or": []bson.M{
					{"students": bson.M{"$in": []bson.ObjectId{id}}},
					{"tutor": id},
				},
				"ends_at": bson.M{
					"$gte": from,
					"$lte": to,
				},
			},
		},
		{
			"$lookup": bson.M{
				"from":         "subjects",
				"localField":   "subject",
				"foreignField": "_id",
				"as":           "subject",
			},
		},
		{
			"$unwind": "$subject",
		},
		{
			"$lookup": bson.M{
				"from":         "users",
				"localField":   "tutor",
				"foreignField": "_id",
				"as":           "tutor",
			},
		},
		{
			"$unwind": "$tutor",
		},
		{
			"$lookup": bson.M{
				"from":         "users",
				"localField":   "students",
				"foreignField": "_id",
				"as":           "student",
			},
		},
		{
			"$unwind": "$student",
		},
	}).All(&lessons); err != nil {
		return nil, err
	}
	return lessons, nil
}

func (l *Lessons) SaveCharges(id bson.ObjectId, charge *models.ChargeData) {
	if err := store.GetCollection("lessons").UpdateId(id, bson.M{
		"$set": bson.M{
			"charge": charge,
		},
	}); err != nil {
		logger.Get().Errorf("failed to save lesson charges: %v", err)
		return
	}
	logger.Get().Infof("saved charges for lesson %v: charge: %v", id, charge.ChargeID)
}
