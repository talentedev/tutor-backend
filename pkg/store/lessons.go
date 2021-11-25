package store

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services/models"
	"gitlab.com/learnt/api/pkg/utils/timeline"

	"github.com/jinzhu/copier"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/utils"
	"gopkg.in/mgo.v2/bson"
)

// LessonState int for enum
type LessonState int

// States that a lesson can be in
const (
	LessonBooked LessonState = iota
	LessonConfirmed
	LessonProgress
	LessonCompleted
	LessonCancelled
)

type LessonType byte

const (
	LessonDefault LessonType = iota
	LessonRecurrent
	LessonInstant
)

// LessonsStore struct to attach functions for lessons
type LessonsStore struct{}

// GetLessonsStore returns in inited LessonStore
func GetLessonsStore() *LessonsStore { return &LessonsStore{} }

// LessonStateData data gathered when a state is changed
type LessonStateData struct {
	State LessonState `json:"state" bson:"state"`
	Time  time.Time   `json:"time" bson:"time"`
	Data  interface{} `json:"data" bson:"data"`
}

// LessonChangeProposal contains info about changed items in a lesson.
type LessonChangeProposal struct {
	// User represents the ID of the user who requested the change.
	User bson.ObjectId `json:"user,omitempty" bson:"user,omitempty"`

	// Subject is the ID of the requested subject.
	Subject bson.ObjectId `json:"subject,omitempty" bson:"subject,omitempty"`

	// Meet is the requested meeting place.
	Meet Meet `json:"meet,omitempty" bson:"meet,omitempty"`
	// Location is the requested location.
	Location string `json:"location,omitempty" bson:"location,omitempty"`

	// StartsAt is the time when the lesson should start.
	StartsAt time.Time `json:"starts_at,omitempty" bson:"starts_at,omitempty"`
	// EndsAt is the time when the lesson should end.
	EndsAt time.Time `json:"ends_at,omitempty" bson:"ends_at,omitempty"`

	// CreatedAt is the date and time of when the change was requested.
	CreatedAt time.Time `json:"created_at,omitempty" bson:"created_at,omitempty"`
}

// Apply takes a change proposal and applies it
func (lcp *LessonChangeProposal) Apply(lessonID bson.ObjectId) (*LessonMgo, error) {
	var lesson LessonMgo
	if err := GetCollection("lessons").FindId(lessonID).One(&lesson); err != nil {
		return nil, errors.Wrap(err, "couldn't get lesson")
	}

	if err := lesson.SetSubject(lcp.Subject); err != nil {
		return nil, errors.Wrap(err, "couldn't set subject")
	}

	if err := lesson.SetMeetAndLocation(lcp.Meet, lcp.Location); err != nil {
		return nil, errors.Wrap(err, "couldn't set meet & location")
	}

	if err := lesson.SetTimes(lcp.StartsAt, lcp.EndsAt); err != nil {
		return nil, errors.Wrap(err, "couldn't set times")
	}

	return &lesson, nil
}

// DTO fills out the IDs into structs from a database type
func (lcp *LessonChangeProposal) DTO() (*LessonChangeProposalDTO, error) {
	var user *UserMgo
	if err := GetCollection("users").FindId(lcp.User).One(&user); err != nil {
		return nil, errors.Wrap(err, "couldn't get user")
	}

	var subject *Subject
	if err := GetCollection("subjects").FindId(lcp.Subject).One(&subject); lcp.Subject.Hex() != "" && err != nil {
		return nil, errors.Wrap(err, "couldn't get subject")
	}

	return &LessonChangeProposalDTO{
		User:      user.Dto(true),
		Subject:   subject,
		Meet:      lcp.Meet,
		Location:  lcp.Location,
		StartsAt:  lcp.StartsAt,
		EndsAt:    lcp.EndsAt,
		CreatedAt: lcp.CreatedAt,
	}, nil
}

// LessonChangeProposalDTO is a lesson change proposal with IDs expanded
type LessonChangeProposalDTO struct {
	User      *UserDto  `json:"user"`
	Subject   *Subject  `json:"subject"`
	Meet      Meet      `json:"meet"`
	Location  string    `json:"location"`
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	CreatedAt time.Time `json:"created_at"`
}

// LessonMgo is the database version of a lesson
type LessonMgo struct {
	ID bson.ObjectId `json:"_id" bson:"_id"`

	Subject bson.ObjectId `json:"subject" bson:"subject"`

	StartsAt time.Time  `json:"starts_at" bson:"starts_at"`
	EndsAt   time.Time  `json:"ends_at" bson:"ends_at"`
	EndedAt  *time.Time `json:"ended_at,omitempty" bson:"ended_at,omitempty"`

	Meet     Meet   `json:"meet" bson:"meet"`
	Location string `json:"location" bson:"location"`

	Tutor    bson.ObjectId   `json:"tutor" bson:"tutor"`
	Students []bson.ObjectId `json:"students" bson:"students"`
	Rate     float32         `json:"rate" bson:"rate"`

	State         LessonState       `json:"state" bson:"state"`
	StateTimeline []LessonStateData `json:"state_timeline" bson:"state_timeline"`

	Accepted []bson.ObjectId `json:"accepted" bson:"accepted"`

	Room *bson.ObjectId `json:"room" bson:"room"`

	Recurrent      bool           `json:"recurrent" bson:"recurrent"`
	RecurrentCount int            `json:"recurrent_count" bson:"recurrent_count"`
	RecurrentID    *bson.ObjectId `json:"recurrent_id" bson:"recurrent_id"`

	Type LessonType `json:"type" bson:"type"`

	Notifications struct {
		OneDayBefore           bool `json:"-" bson:"one_day_before,omitempty"`
		ThirtiethMinutesBefore bool `json:"-" bson:"thirtieth_minutes_before,omitempty"`
		TenMinutesBefore       bool `json:"-" bson:"ten_minutes_before,omitempty"`
	} `json:"-" bson:"notifications,omitempty"`

	ChangeProposals []LessonChangeProposal `json:"change_proposals" bson:"change_proposals"`

	// AutoBillable bool `json:"auto_billable" bson:"auto_billable"`

	CreatedAt time.Time          `json:"created_at" bson:"created_at"`
	Charge    *models.ChargeData `json:"charge,omitempty" bson:"charge,omitempty"`

	updatemux *sync.Mutex `bson:"-"`
}

func (l LessonMgo) IsInstantSession() bool {
	return l.Type == LessonInstant
}

func (l LessonMgo) Clone() LessonMgo {
	out := LessonMgo{}
	copier.Copy(&out, l)
	return out
}

// IsNotified gets true if 1d,30m,10m before notifications sent
func (l LessonMgo) IsNotified() bool {
	return l.Notifications.OneDayBefore && l.Notifications.ThirtiethMinutesBefore && l.Notifications.TenMinutesBefore
}

func (l LessonMgo) IsConfirmed() bool {
	return l.State == LessonConfirmed
}

func (l LessonMgo) GetParticipants() (participants []*UserMgo) {
	participants = make([]*UserMgo, 0)
	participants = append(participants, l.GetTutor())
	for _, id := range l.Students {
		if student, err := getUser(id); err == nil {
			participants = append(participants, student)
		}
	}
	return
}

func (l LessonMgo) GetTimelineSlot() timeline.SlotProvider {

	occurence := timeline.None

	if l.Recurrent {
		occurence = timeline.Weekly
	}
	var endsAt = l.EndsAt

	return &timeline.Slot{
		ID:        l.ID.Hex(),
		From:      l.StartsAt.Add(0),
		To:        endsAt.Add(0),
		Occurence: occurence,
	}
}

func (l LessonMgo) GetFinalLessonEndsAt() time.Time {
	endsAt := l.EndsAt

	if l.RecurrentCount > 1 {
		endsAt = endsAt.AddDate(0, 0, 7*(l.RecurrentCount-1))
	}

	return endsAt
}

func (l LessonMgo) GetTo() time.Time {
	return *l.EndedAt
}

func (l *LessonMgo) GetOccurence() timeline.Occurence {
	if l.Recurrent {
		return timeline.Weekly
	}
	return timeline.None
}

// LessonDto similar to database struct but with structs foreign IDs
type LessonDto struct {
	ID bson.ObjectId `json:"_id" bson:"_id"`

	Subject Subject `json:"subject" bson:"subject"`

	StartsAt time.Time  `json:"starts_at" bson:"starts_at"`
	EndsAt   time.Time  `json:"ends_at" bson:"ends_at"`
	EndedAt  *time.Time `json:"ended_at,omitempty" bson:"ended_at,omitempty"`

	Meet     Meet   `json:"meet" bson:"meet"`
	Location string `json:"location" bson:"location"`

	Tutor    PublicUserDto   `json:"tutor" bson:"tutor"`
	Students []PublicUserDto `json:"students" bson:"students"`
	Student  *PublicUserDto  `json:"student,omitempty" bson:"student"`
	Rate     float32         `json:"rate" bson:"rate"`

	State         LessonState       `json:"state" bson:"state"`
	StateTimeline []LessonStateData `json:"state_timeline" bson:"state_timeline"`

	Accepted []PublicUserDto `json:"accepted" bson:"accepted"`

	Room *bson.ObjectId `json:"room" bson:"room"`

	Recurrent   bool           `json:"recurrent" bson:"recurrent"`
	RecurrentID *bson.ObjectId `json:"recurrent_id" bson:"recurrent_id"`

	Notifications struct {
		OneDayBefore           bool `json:"-" bson:"one_day_before,omitempty"`
		ThirtiethMinutesBefore bool `json:"-" bson:"thirtieth_minutes_before,omitempty"`
		TenMinutesBefore       bool `json:"-" bson:"ten_minutes_before,omitempty"`
	} `json:"-" bson:"notifications,omitempty"`

	ChangeProposals []LessonChangeProposalDTO `json:"change_proposals" bson:"change_proposals"`

	AutoBillable bool `json:"auto_billable" bson:"auto_billable"`

	CreatedAt  time.Time          `json:"created_at" bson:"created_at"`
	LessonType LessonType         `json:"lesson_type" bson:"type"`
	Charge     *models.ChargeData `json:"charge,omitempty" bson:"charge,omitempty"`
}

// DTO fills in the IDs from the database value with stucts
func (l *LessonMgo) DTO() (*LessonDto, error) {
	var tutor UserMgo

	if err := GetCollection("users").FindId(l.Tutor).One(&tutor); err != nil {
		return nil, errors.Wrap(err, "couldn't get tutor")
	}

	students, err := l.StudentsDto()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get students")
	}

	subject, err := l.GetSubject()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get subject")
	}

	acceptedUsers := make([]UserMgo, len(l.Accepted))
	if err = GetCollection("users").FindId(bson.M{"$in": l.Accepted}).All(&acceptedUsers); err != nil {
		return nil, errors.Wrap(err, "couldn't expand accepted users")
	}
	acceptedUsersDto := make([]PublicUserDto, len(acceptedUsers))
	for _, user := range acceptedUsers {
		acceptedUsersDto = append(acceptedUsersDto, *user.ToPublicDto())
	}

	proposals := make([]LessonChangeProposalDTO, len(l.ChangeProposals))
	for i, lcp := range l.ChangeProposals {
		dto, err := lcp.DTO()
		if err != nil {
			return nil, errors.Wrap(err, "couldn't get change proposal")
		}
		proposals[i] = *dto
	}

	return &LessonDto{
		ID:              l.ID,
		Subject:         *subject,
		StartsAt:        l.StartsAt,
		EndsAt:          l.EndsAt,
		EndedAt:         l.EndedAt,
		Meet:            l.Meet,
		Location:        l.Location,
		Tutor:           *tutor.ToPublicDto(),
		Students:        students,
		Rate:            l.Rate,
		State:           l.State,
		StateTimeline:   l.StateTimeline,
		Accepted:        acceptedUsersDto,
		Room:            l.Room,
		Recurrent:       l.Recurrent,
		RecurrentID:     l.RecurrentID,
		Notifications:   l.Notifications,
		ChangeProposals: proposals,
		CreatedAt:       l.CreatedAt,
		Charge:          l.Charge,
	}, nil
}

// LessonsToDTO does the lookup DTO lookups for many lessons caching along the way
func LessonsToDTO(lessons []LessonMgo) []*LessonDto {
	subjects := map[bson.ObjectId]*Subject{}
	users := map[bson.ObjectId]*PublicUserDto{}

	var err error
	lessonsDTO := make([]*LessonDto, len(lessons))
	for i, l := range lessons {

		if _, ok := subjects[l.Subject]; !ok {
			subjects[l.Subject], err = l.GetSubject()
			if err != nil {
				err = errors.Wrap(err, "could not get subject of lesson")
				core.PrintError(err, "lessonToDTO")
			}
		}

		userCollection := GetCollection("users")
		userIDs := append(l.Students, l.Tutor)
		for _, v := range userIDs {
			if _, ok := users[v]; !ok {
				var user UserMgo
				if err := userCollection.FindId(v).One(&user); err != nil {
					err = errors.Wrap(err, "could not get user for lesson")
					core.PrintError(err, "lessonToDTO")
					continue
				}
				u := user.ToPublicDto()
				u.Tutoring = nil
				users[v] = u
			}
		}

		students := make([]PublicUserDto, len(l.Students))
		for i, v := range l.Students {
			students[i] = *users[v]
		}
		acceptedUsers := make([]PublicUserDto, len(l.Accepted))
		for i, v := range l.Accepted {
			acceptedUsers[i] = *users[v]
		}

		proposals := make([]LessonChangeProposalDTO, len(l.ChangeProposals))
		for i, lcp := range l.ChangeProposals {
			dto, err := lcp.DTO()
			if err != nil {
				err = errors.Wrap(err, "could not get change proposal for lesson")
				core.PrintError(err, "lessonToDTO")
				continue
			}
			proposals[i] = *dto
		}

		lessonsDTO[i] = &LessonDto{
			ID:              l.ID,
			Subject:         *subjects[l.Subject],
			StartsAt:        l.StartsAt,
			EndsAt:          l.EndsAt,
			EndedAt:         l.EndedAt,
			Meet:            l.Meet,
			Location:        l.Location,
			Tutor:           *users[l.Tutor],
			Students:        students,
			Rate:            l.Rate,
			State:           l.State,
			StateTimeline:   l.StateTimeline,
			Accepted:        acceptedUsers,
			Room:            l.Room,
			Recurrent:       l.Recurrent,
			Notifications:   l.Notifications,
			ChangeProposals: proposals,
			CreatedAt:       l.CreatedAt,
			Charge:          l.Charge,
		}
	}

	return lessonsDTO
}

func (l *LessonMgo) String() string {
	return fmt.Sprintf("[Lesson %s %s]", l.ID.Hex(), l.WhenFormatted())
}

func (l *LessonMgo) OtherParticipants(uID bson.ObjectId) (other []*UserMgo, err error) {

	other = make([]*UserMgo, 0)

	if l.Tutor.Hex() != uID.Hex() {
		tutor, err := getUser(l.Tutor)
		if err != nil {
			return other, errors.Wrap(err, "Failed to retrieve tutor from lesson")
		}
		other = append(other, tutor)
	}

	for _, studentID := range l.Students {
		if studentID.Hex() != uID.Hex() {
			student, err := getUser(studentID)
			if err != nil {
				return other, errors.Wrap(err, "Failed to retrive student for lesson")
			}
			other = append(other, student)
		}
	}

	return
}

// OtherParticipantIds returns the ID of the participant not passed in
func (l *LessonMgo) OtherParticipantIds(uID bson.ObjectId) (bson.ObjectId, error) {
	if !l.HasUserID(uID) {
		return "", fmt.Errorf("invalid user provided")
	}
	participants := l.GetParticipants()
	for _, participant := range participants {
		if participant.ID.Hex() != uID.Hex() {
			return participant.ID, nil
		}
	}

	return "", fmt.Errorf("couldn't get tutor or student")
}

// CanBeModified tells if a lesson can be modified based on state
func (l *LessonMgo) CanBeModified() bool {
	switch l.State {
	case LessonProgress, LessonCompleted, LessonCancelled:
		return false
	default:
		return true
	}
}

// HoursUntilStarts returns the number of hours until the lesson starts.
func (l *LessonMgo) HoursUntilStarts() int {
	return int(l.StartsAt.Sub(time.Now()).Hours())
}

// StartsWithinToday true if the lesson will start within 24 hours.
func (l *LessonMgo) StartsWithin24Hours() bool {
	return int(l.StartsAt.Sub(time.Now()).Hours()) < 24
}

// Duration returns the lesson's duration.
func (l *LessonMgo) Duration() time.Duration {
	return l.EndsAt.Sub(l.StartsAt)
}

// StartDateTimeFormatted returns the lesson's start date in "Jan 1, 2006 15:04:05" format.
func (l *LessonMgo) StartsAtDateTimeFormatted() string {
	return utils.TimeBasicShortDateTimeFormat(l.StartsAt)
}

// WhenFormatted returns the starting time of the lesson, but formatted.
func (l *LessonMgo) WhenFormattedWithTimezone(timezone string) string {
	return utils.FormatTimeWithTimezone(l.StartsAt, timezone)
}

// WhenFormatted returns the starting time of the lesson, but formatted.
func (l *LessonMgo) WhenFormatted() string {
	return utils.FormatTime(l.StartsAt)
}

// WhenFormatted returns the starting time of the lesson, but formatted.
func (l *LessonMgo) WhenFormatTime(format string) string {
	return utils.GeneralFormatTime(format, l.StartsAt)
}

// FetchSubjectName returns the lesson's subject name.
func (l *LessonMgo) FetchSubjectName() string {
	var sub Subject
	if err := GetCollection("subjects").FindId(l.Subject).One(&sub); err != nil {
		panic(err)
	}
	return utils.CapitalizeFirstWord(sub.Name)
}

// EndsFormatted returns the ending time of the lesson, but formatted.
func (l *LessonMgo) EndsFormatted() string {
	return utils.FormatTime(l.EndsAt)
}

// GetTutor returns the lesson's tutor.
func (l *LessonMgo) GetTutor() (tutor *UserMgo) {
	if err := GetCollection("users").FindId(l.Tutor).One(&tutor); err != nil {
		panic(err)
	}
	return
}

// TutorDto returns the lesson's tutor.
func (l *LessonMgo) TutorDto() *UserDto {
	return l.GetTutor().Dto()
}

// StudentsDto returns the lesson's students.
func (l *LessonMgo) StudentsDto() (students []PublicUserDto, err error) {
	mgos := make([]UserMgo, len(l.Students))
	if err = GetCollection("users").FindId(bson.M{"$in": l.Students}).All(&mgos); err != nil {
		return
	}
	for i := range mgos {
		students = append(students, *mgos[i].ToPublicDto())
	}
	return
}

// StudentsNames returns the lesson's students' names.
func (l *LessonMgo) StudentsNames(markdown bool) string {
	names := make([]string, 0)
	students := make([]UserMgo, 0)

	GetCollection("users").FindId(bson.M{"$in": l.Students}).All(&students)

	for _, student := range students {
		name := student.Name()

		if markdown {
			name = fmt.Sprintf("[%s](@Route(/users/%s))", student.Name(), student.ID.Hex())
		}

		names = append(names, name)
	}

	return strings.Join(names, ", ")
}

// GetSubject returns a Subject from a lesson's subject ID.
func (l *LessonMgo) GetSubject() (s *Subject, err error) {
	err = GetCollection("subjects").FindId(l.Subject).One(&s)
	return
}

func (l *LessonMgo) lockMux() {
	if l.updatemux == nil {
		l.updatemux = &sync.Mutex{}
	}
	l.updatemux.Lock()
}

// SetSubject updates the lesson's subject.
func (l *LessonMgo) SetSubject(s bson.ObjectId) error {
	l.lockMux()
	defer l.updatemux.Unlock()

	l.Subject = s
	err := GetCollection("lessons").UpdateId(l.ID, bson.M{"$set": bson.M{"subject": s}})

	return errors.Wrap(err, "couldn't update lesson subject")
}

// SetMeetAndLocation updates the lesson's meeting place and location.
func (l *LessonMgo) SetMeetAndLocation(m Meet, loc string) error {
	l.lockMux()
	defer l.updatemux.Unlock()

	l.Meet = m
	l.Location = loc

	err := GetCollection("lessons").UpdateId(l.ID, bson.M{"$set": bson.M{
		"meet":     m,
		"location": loc,
	}})

	return errors.Wrap(err, "couldn't update lesson meet and location")
}

// SetTimes sets the start and end time on the lesson.
func (l *LessonMgo) SetTimes(startsAt, endsAt time.Time) error {
	l.lockMux()
	defer l.updatemux.Unlock()

	l.StartsAt = startsAt
	l.EndsAt = endsAt

	err := GetCollection("lessons").UpdateId(l.ID, bson.M{"$set": bson.M{
		"starts_at": l.StartsAt,
		"ends_at":   l.EndsAt,
	}})

	return errors.Wrap(err, "couldn't update lesson times")
}

// ProposeChange saves a change proposal
func (l *LessonMgo) ProposeChange(change LessonChangeProposal) error {
	if len(l.ChangeProposals) > 0 && !l.EveryoneAccepted() {
		return errors.New("already proposed a change, awaiting confirmation")
	}

	if !l.CanBeModified() {
		return errors.New("can't modify lesson")
	}

	l.lockMux()
	defer l.updatemux.Unlock()

	l.Accepted = []bson.ObjectId{
		change.User, // user who proposed the change
	}

	err := GetCollection("lessons").UpdateId(l.ID, bson.M{
		"$push": bson.M{
			"change_proposals": change,
		},
		"$set": bson.M{
			"accepted": l.Accepted,
		},
	})

	if err != nil {
		return errors.New("couldn't update lesson to add changes")
	}

	return nil
}

// SetState sets the lesson's state, and updates its data, if provided.
func (l *LessonMgo) SetState(state LessonState, all *bool, dataOpts ...interface{}) (err error) {
	var data interface{}
	if len(dataOpts) > 0 {
		data = dataOpts[0]
	}

	l.lockMux()
	defer l.updatemux.Unlock()

	idsToUpdate := []bson.ObjectId{l.ID}
	if all != nil && *all && l.RecurrentID != nil {
		futureLessons := GetLessonsStore().GetFutureRecurringLessons(*l.RecurrentID, l.StartsAt)
		for _, lesson := range futureLessons {
			idsToUpdate = append(idsToUpdate, lesson.ID)
		}
	}

	logger.Get().Infof("IDs to Update: %#v", idsToUpdate)
	for _, id := range idsToUpdate {
		err = GetCollection("lessons").UpdateId(id, bson.M{
			"$set": bson.M{"state": state},
			"$push": bson.M{
				"state_timeline": LessonStateData{State: state, Time: time.Now(), Data: data},
			},
		})
	}

	return
}

// SetState sets the lesson's state, and updates its data, if provided.
func (l *LessonMgo) SetEndsAt(endsAt time.Time) (err error) {
	err = GetCollection("lessons").UpdateId(l.ID, bson.M{
		"$set": bson.M{"ends_at": endsAt},
	})

	return
}

// HasUser returns whether the lesson has the specified user, or not.
func (l *LessonMgo) HasUser(user *UserMgo) bool {
	return l.HasUserID(user.ID)
}

// HasUserID returns whether the lesson has the specified user by ID, or not.
func (l *LessonMgo) HasUserID(userID bson.ObjectId) bool {
	isTutor := l.Tutor.Hex() == userID.Hex()
	var isStudent bool
	for _, s := range l.Students {
		if s.Hex() == userID.Hex() {
			isStudent = true
			break
		}
	}
	return isTutor || isStudent
}

// EveryoneAccepted returns whether everyone in the lesson has accepted the lesson, or not.
func (l *LessonMgo) EveryoneAccepted() bool {
	return len(l.Students)+1 /*tutor*/ == len(l.Accepted)
}

// AcceptBy sets the accepted state by the specified user. Returns whether everyone accepted the lesson,
// and an error if the user already accepted it, or if the user isn't a participant of the lesson.
func (l *LessonMgo) AcceptBy(user *UserMgo) (everyone bool, err error) {
	l.lockMux()
	defer l.updatemux.Unlock()

	for _, student := range l.Accepted {
		if student.Hex() == user.ID.Hex() {
			return l.EveryoneAccepted(), nil
		}
	}

	if !l.HasUser(user) {
		return false, errors.New("only lesson participants can accept the lesson")
	}

	l.Accepted = append(l.Accepted, user.ID)

	err = GetCollection("lessons").UpdateId(l.ID, bson.M{
		"$set": bson.M{
			"accepted": l.Accepted,
		},
	})

	return l.EveryoneAccepted(), err
}

// Reject cancels a lesson with a rejection
func (l *LessonMgo) Reject(user *UserMgo, reason string) (err error) {
	// TODO: Case when one student from many reject the lesson
	// Remove from students
	// If lesson has fewer than X students reject the lesson

	if !l.HasUser(user) {
		return errors.New("only lesson participants can reject the lesson")
	}

	if l.State == LessonCancelled {
		return errors.New("lesson already cancelled")
	}

	err = l.SetState(LessonCancelled, nil, map[string]interface{}{
		"user":   user.ID,
		"reason": reason,
	})

	return errors.Wrap(err, "Fail to set cancelled state")
}

// SetRecurrent updates a lesson to be recurrent
func (l *LessonMgo) SetRecurrent(r bool, count int) error {
	if l.Recurrent == r {
		return nil
	}

	l.Recurrent = r
	endsAt := l.EndsAt
	for i := 1; i < count; i++ {
		endsAt.AddDate(0, 0, 1)
	}

	err := GetCollection("lessons").UpdateId(l.ID, bson.M{"$set": bson.M{"recurrent": r, "endsAt": endsAt}})
	return errors.Wrap(err, "couldn't update lesson in database")
}

// SaveRoom updates the lesson's room.
func (l *LessonMgo) SetRoom(id bson.ObjectId) (err error) {
	return GetCollection("lessons").UpdateId(l.ID, bson.M{
		"$set": bson.M{
			"room": id,
		},
	})
}

const (
	LessonNotificationOneDay        = "one_day_before"
	LessonNotificationThirtyMinutes = "thirtieth_minutes_before"
	LessonNotificationTenMinutes    = "ten_minutes_before"
	LessonNotificationBefore        = "before"
)

func (l *LessonMgo) SetIsNotified(kind string) (err error) {
	return GetCollection("lessons").UpdateId(l.ID,
		bson.M{
			"$set": bson.M{"notifications." + string(kind): true},
		},
	)
}

// Get finds a lesson by an ID
func (L *LessonsStore) Get(id bson.ObjectId) (lesson *LessonMgo, exist bool) {
	exist = GetCollection("lessons").FindId(id).One(&lesson) == nil
	return
}

// Get finds a lesson by an ID
func (L *LessonsStore) GetCurrentRunningLessons(offset int, limit int) (*PaginatedLessons, error) {
	query := GetCollection("lessons").Find(bson.M{"$and": []bson.M{
		{"ends_at": bson.M{"$gte": time.Now()}},
		{"starts_at": bson.M{"$lte": time.Now()}},
	}}).Sort("-created_at")

	length, err := query.Count()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get lessons from database")
	}

	query = query.Skip(offset).Limit(limit)
	var l []LessonMgo
	err = query.All(&l)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get lessons from database")
	}

	pl := &PaginatedLessons{
		Lessons: l,
		Length:  length,
	}

	return pl, nil
}

func (L *LessonsStore) GetLessons() (lessons []LessonMgo) {
	GetCollection("lessons").Find(bson.M{"$or": []bson.M{
		{"recurrent": true, "ends_at": bson.M{"$gt": time.Now()}},
		{"recurrent": false, "starts_at": bson.M{"$gte": time.Now()}},
	}}).Sort("-created_at").All(&lessons)
	return
}

func (L *LessonsStore) GetDefaultLessons() (lessons []LessonMgo) {
	GetCollection("lessons").Find(bson.M{"$or": []bson.M{
		{"recurrent": true},
		{"recurrent": false},
		{"type": LessonDefault},
		{"type": LessonRecurrent},
	}}).Sort("starts_at").All(&lessons)
	return
}

func (L *LessonsStore) GetInstantLessons() (lessons []LessonMgo) {
	GetCollection("lessons").Find(bson.M{"type": LessonInstant}).Sort("starts_at").All(&lessons)
	return
}

func (L *LessonsStore) GetLessonsAtTime(startTime time.Time) (lessons []LessonMgo) {
	query := bson.M{"starts_at": bson.M{"$eq": startTime}}
	logger.Get().Infof("get lessons at time query: %+v", query)
	GetCollection("lessons").Find(query).All(&lessons)
	return
}

func (L *LessonsStore) GetLessonsWithin24Hours(start time.Time, end time.Time) (lessons []LessonMgo) {
	query := bson.M{"$and": []bson.M{
		{"starts_at": bson.M{"$lte": end}},
		{"starts_at": bson.M{"$gte": start}},
	}}

	logger.Get().Infof("get lessons at time query: %+v", query)
	GetCollection("lessons").Find(query).All(&lessons)
	return
}

func (L *LessonsStore) GetFutureRecurringLessons(recurrentID bson.ObjectId, startDate time.Time) (lessons []LessonMgo) {
	GetCollection("lessons").Find(bson.M{"$and": []bson.M{
		{"recurrent_id": recurrentID},
		{"starts_at": bson.M{"$gte": startDate}},
	}}).Sort("-created_at").All(&lessons)
	return
}

func (L *LessonsStore) GetAllUserLessons(user *UserMgo) ([]LessonMgo, error) {
	var lessons []LessonMgo
	if err := GetCollection("lessons").Find(bson.M{"$or": []bson.M{
		{"tutor": user.ID},
		{"students": user.ID},
	}}).Sort("-created_at").All(&lessons); err != nil {
		return nil, errors.Wrap(err, "couldn't get lessons from database")
	}

	return lessons, nil
}

// PaginatedLessons has a page of lessons with the count of all lessons matching the query
type PaginatedLessons struct {
	Lessons []LessonMgo
	Length  int
}

// GetAllUserLessonsPaginated gets a paginated list of lessons for a user
func (L *LessonsStore) GetAllUserLessonsPaginated(queryParts []bson.M, offset, limit int) (*PaginatedLessons, error) {

	var q bson.M
	if len(queryParts) == 1 {
		q = queryParts[0]
	} else {
		q = bson.M{"$and": queryParts}
	}

	query := GetCollection("lessons").Find(q).Sort("-state", "-starts_at")
	length, err := query.Count()
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get lessons from database")
	}

	var l []LessonMgo
	query = query.Skip(offset).Limit(limit)
	// PrintExplaination("lessons", query)
	err = query.All(&l)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get lessons from database")
	}

	pl := &PaginatedLessons{
		Lessons: l,
		Length:  length,
	}
	return pl, nil
}

// GetUserLessons gets all the lessons of a user
func (L *LessonsStore) GetUserLessons(user *UserMgo, lessThanEqual bool, state LessonState) (lessons []*LessonDto) {
	lessons = make([]*LessonDto, 0)

	var stateQuery bson.M
	if lessThanEqual {
		stateQuery = bson.M{"$lte": state}
	} else {
		stateQuery = bson.M{"$eq": state}
	}

	toPipe := []bson.M{{
		"$match": bson.M{
			"$or": []bson.M{
				{"tutor": user.ID},
				{"students": user.ID},
			},
			"state": stateQuery,
		}},
		{
			"$project": bson.M{
				"_id":              1,
				"subject":          1,
				"starts_at":        1,
				"ends_at":          1,
				"meet":             1,
				"location":         1,
				"tutor":            1,
				"students":         1,
				"rate":             1,
				"state":            1,
				"state_timeline":   1,
				"accepted":         1,
				"room":             1,
				"change_proposals": 1,
				"auto_billable":    1,
				"created_at":       1,
				"recurrent":        1,
			},
		},
	}

	var lessonMgos []*LessonMgo
	if err := GetCollection("lessons").Pipe(toPipe).All(&lessonMgos); err != nil {
		logger.Get().Errorf("couldn't get db lessons: %v", err)
		return lessons
	}

	for _, l := range lessonMgos {
		dto, err := l.DTO()
		if err != nil {
			logger.Get().Errorf("lesson conversion error: %v", err)
		}
		lessons = append(lessons, dto)
	}

	for _, l := range lessons {
		l.StartsAt = l.StartsAt.UTC()
		l.EndsAt = l.EndsAt.UTC()
	}

	return
}
