package store

/*
type instantSessionsStore struct{}

// InstantSessionsStore gets struct with instant session functions
func InstantSessionsStore() *instantSessionsStore { return &instantSessionsStore{} }

// InstantSessionStatus is a type for a status enum
type InstantSessionStatus byte

// enum of instant session statuses
const (
	InstantSessionCreated InstantSessionStatus = iota
	InstantSessionAccepted
	InstantSessionRejected
	InstantSessionCompleted
	InstantSessionFailed
)

// InstantSessionEndReason is a type for a end reson enum
type InstantSessionEndReason byte

// enum of instant session end reasons
const (
	InstantSessionTutorReject InstantSessionEndReason = iota
	InstantSessionStudentReject
	InstantSessionSystemReject
	InstantSessionTutorQuit
	InstantSessionStudentQuit
)

// InstantSession is the struct of the instant lessons.
type InstantSession struct {
	ID        bson.ObjectId            `json:"_id" bson:"_id"`
	CreatedAt time.Time                `json:"created_at" bson:"created_at"`
	StartsAt  *time.Time               `json:"starts_at" bson:"starts_at"`
	EndedAt   *time.Time               `json:"ended_at" bson:"ended_at"`
	Tutor     bson.ObjectId            `json:"tutor" bson:"tutor"`
	Student   bson.ObjectId            `json:"student" bson:"student"`
	Status    InstantSessionStatus     `json:"status" bson:"status"`
	EndReason *InstantSessionEndReason `json:"end_reason" bson:"end_reason"`
	Room      *bson.ObjectId           `json:"room" bson:"room"`
	Subject   bson.ObjectId            `json:"subject" bson:"subject"`
}

// InstantSessionDTO includes structs for users instead of IDs
type InstantSessionDTO struct {
	ID        bson.ObjectId            `json:"id"`
	CreatedAt time.Time                `json:"created_at"`
	StartsAt  *time.Time               `json:"starts_at"`
	EndedAt   *time.Time               `json:"ended_at"`
	Tutor     *UserDto                 `json:"tutor"`
	Student   *UserDto                 `json:"student"`
	Status    InstantSessionStatus     `json:"status"`
	EndReason *InstantSessionEndReason `json:"end_reason"`
	Room      *bson.ObjectId           `json:"room"`
	Subject   bson.ObjectId            `json:"subject" bson:"subject"`
}

// DTO fills in IDs for the database object
func (i *InstantSession) DTO() (*InstantSessionDTO, error) {
	var tutor UserMgo
	if err := GetCollection("users").FindId(i.Tutor).One(&tutor); err != nil {
		return nil, errors.Wrap(err, "couldn't get tutor")
	}

	var student UserMgo
	if err := GetCollection("users").FindId(i.Student).One(&student); err != nil {
		return nil, errors.Wrap(err, "couldn't get student")
	}

	return &InstantSessionDTO{
		ID: i.ID,

		CreatedAt: i.CreatedAt,
		StartsAt:  i.StartsAt,
		EndedAt:   i.EndedAt,

		Tutor:   tutor.Dto(true),
		Student: student.Dto(true),

		Status:    i.Status,
		EndReason: i.EndReason,

		Room:    i.Room,
		Subject: i.Subject,
	}, nil
}

// InstantSessionsToDTO does the lookup DTO lookups for many instant session caching along the way
func InstantSessionsToDTO(instantLessons []InstantSession) []*InstantSessionDTO {
	users := map[bson.ObjectId]*UserDto{}
	instantSessionDTOs := make([]*InstantSessionDTO, len(instantLessons))

	for i, l := range instantLessons {
		userCollection := GetCollection("users")
		userIDs := []bson.ObjectId{l.Student, l.Tutor}
		for _, v := range userIDs {
			if _, ok := users[v]; !ok {
				var user UserMgo
				if err := userCollection.FindId(v).One(&user); err != nil {
					err = errors.Wrap(err, "could not get user for instant lesson")
					core.PrintError(err, "lessonToDTO")
					continue
				}
				u := user.Dto()
				u.Tutoring = nil
				users[v] = u
			}
		}

		instantSessionDTOs[i] = &InstantSessionDTO{
			ID: l.ID,

			CreatedAt: l.CreatedAt,
			StartsAt:  l.StartsAt,
			EndedAt:   l.EndedAt,

			Tutor:   users[l.Tutor],
			Student: users[l.Student],

			Status:    l.Status,
			EndReason: l.EndReason,

			Room:    l.Room,
			Subject: l.Subject,
		}
	}

	return instantSessionDTOs
}

// HasParticipant returns whether a user is a participant in the instant session.
func (i *InstantSession) HasParticipant(userID bson.ObjectId) bool {
	return i.Tutor.Hex() == userID.Hex() || i.Student.Hex() == userID.Hex()
}

// Ended returns whether the instant session ended
func (i *InstantSession) Ended() bool {
	if i == nil {
		return true
	}
	return i.EndedAt != nil || i.EndReason != nil || i.Status == InstantSessionCompleted
}

// SetStatus updates the status on an instant session
func (i *InstantSession) SetStatus(s InstantSessionStatus, r ...InstantSessionEndReason) (*InstantSession, error) {
	if i.Ended() {
		return i, errors.New("can't alter an ended session")
	}

	i.Status = s

	if len(r) > 0 {
		i.EndReason = &r[0]
	}

	now := time.Now().UTC()
	switch s {
	case InstantSessionCreated:
	case InstantSessionAccepted:
		i.StartsAt = &now
	case InstantSessionCompleted, InstantSessionFailed:
		i.EndedAt = &now
	case InstantSessionRejected:
		i.EndedAt = &now
		var reason = InstantSessionTutorReject
		if len(r) > 0 {
			reason = r[0]
		}
		i.EndReason = &reason
	}

	return i, GetCollection("instant_sessions").UpdateId(i.ID, bson.M{"$set": bson.M{
		"status":     i.Status,
		"starts_at":  i.StartsAt,
		"ended_at":   i.EndedAt,
		"end_reason": i.EndReason,
	}})
}

// SetEndReason updates the end reason on the instant session
func (i *InstantSession) SetEndReason(r InstantSessionEndReason) error {
	reason := r
	i.EndReason = &reason
	return GetCollection("instant_sessions").UpdateId(i.ID, bson.M{"$set": bson.M{"end_reason": i.EndReason}})
}

// SetRoom updates the room on the instant session
func (i *InstantSession) SetRoom(r *bson.ObjectId) error {
	if i.Ended() {
		return errors.New("can't alter an ended session")
	}

	i.Room = r

	return GetCollection("instant_sessions").UpdateId(i.ID, bson.M{"$set": bson.M{"room": i.Room}})
}

// Duration gets the duration of an instant lesson
func (i *InstantSession) Duration() time.Duration {
	if i.StartsAt == nil || i.EndedAt == nil {
		return 0
	}

	return i.EndedAt.Sub(*i.StartsAt)
}

// ByID returns the instant session with the specified id, or an error if it fails
func (i *instantSessionsStore) ByID(id bson.ObjectId) (session *InstantSession, err error) {
	err = GetCollection("instant_sessions").FindId(id).One(&session)
	return
}

*/
