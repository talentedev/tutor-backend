package services

/*
type sessions struct {
	*mgo.Collection
}

// InstantSessions returns the instant sessions collection.
func InstantSessions() *sessions {
	return &sessions{
		store.GetCollection("instant_sessions"),
	}
}

// CheckTimeout adds a timeout after which sets the session as failed
// if the state wasn't updated from created.
func (s *sessions) CheckTimeout(id bson.ObjectId) {
	t := time.NewTicker(2 * time.Minute)
	go func() {
		select {
		case <-t.C:
			t.Stop()

			var i *store.InstantSession
			InstantSessions().FindId(id).One(&i)

			if i.Ended() || i.Status != store.InstantSessionCreated {
				return
			}

			now := time.Now().UTC()

			i.Status = store.InstantSessionFailed
			var reason = store.InstantSessionSystemReject
			i.EndReason = &reason
			i.EndedAt = &now

			InstantSessions().UpdateId(i.ID, bson.M{"$set": bson.M{
				"status":     i.Status,
				"end_reason": i.EndReason,
				"ended_at":   i.EndedAt,
			}})
		}
	}()
}

// InstantSessionRequest is the request that is sent to the API for instantiating an instant session.
type InstantSessionRequest struct {
	Tutor   bson.ObjectId `json:"tutor" binding:"required"`
	Student bson.ObjectId `json:"student" binding:"required"`
	When    time.Time     `json:"when" binding:"required"`
}

// Get returns an instant session with the specified id if exists, otherwise an error.
func (s *sessions) Get(lessonID string) (*store.InstantSession, error) {
	var is *store.InstantSession
	if err := store.GetCollection("instant_sessions").FindId(bson.ObjectIdHex(lessonID)).One(&is); err != nil {
		return nil, errors.Wrap(err, "couldn't get instant session")
	}

	return is, nil
}

// New is the method called by the API route to create an instant session.
func (s *sessions) New(u *store.UserMgo, r *InstantSessionRequest) (*store.InstantSession, error) {
	users := NewUsers()

	tutor, ok := users.ByID(r.Tutor)
	if !ok {
		return nil, errors.New("invalid tutor")
	}

	if !tutor.IsTutor() {
		return nil, errors.New("invalid tutor")
	}

	student, ok := users.ByID(r.Student)
	if !ok {
		return nil, errors.New("invalid student")
	}

	if student.Payments == nil || len(student.Payments.Cards) == 0 {
		return nil, errors.New("student does not a card for payment")
	}

	when := r.When.UTC()

	if !tutor.IsFree(when, 15*time.Minute) {
		return nil, errors.New("tutor is not free")
	}

	if len(tutor.Tutoring.Subjects) == 0 {
		return nil, errors.New("Tutor must have at least one subject defined")
	}

	lesson := store.InstantSession{
		ID: bson.NewObjectId(),

		CreatedAt: when,
		StartsAt:  nil,
		EndedAt:   nil,

		Tutor:   r.Tutor,
		Student: r.Student,

		Status:    store.InstantSessionCreated,
		EndReason: nil,

		Room: nil,

		Subject: tutor.Tutoring.Subjects[0].Subject,
	}

	if err := store.GetCollection("instant_sessions").Insert(lesson); err != nil {
		return nil, errors.Wrap(err, "couldn't insert instant session")
	}

	InstantSessions().CheckTimeout(lesson.ID)

	title := fmt.Sprintf("%s wants to start an instant session", student.Profile.LastName)
	message := "Are you available to start the class right now?"
	action := ""

	notifications.Notify(&notifications.NotifyRequest{
		User:    r.Tutor,
		Type:    notifications.InstantLessonRequest,
		Title:   title,
		Message: message,
		Action:  &action,
		Data:    map[string]interface{}{"lesson": lesson},
	})

	return &lesson, nil
}

func (s *sessions) Accept(u *store.UserMgo, i string) (*store.InstantSession, error) {
	if _, ok := NewUsers().ByID(u.ID); !ok {
		return nil, errors.New("invalid user id")
	}

	lesson, err := store.InstantSessionsStore().ByID(bson.ObjectIdHex(i))
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get instant session")
	}

	if lesson.Ended() {
		return nil, errors.New("can't accept an already ended session")
	}

	if lesson.Status != store.InstantSessionCreated {
		return nil, errors.New("can't accept a modified session")
	}

	if u.ID.Hex() != lesson.Tutor.Hex() {
		return nil, errors.New("invalid tutor accepting request")
	}

	if _, err := lesson.SetStatus(store.InstantSessionAccepted); err != nil {
		return nil, errors.Wrap(err, "couldn't set status to accepted")
	}

	room, err := VCRInstance().CreateInstantRoom(lesson)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't create instant room")
	}

	lesson.Room = &room.ID

	if student, ok := NewUsers().ByID(lesson.Student); ok {
		action := fmt.Sprintf("/class/%s", room.ID)

		notifications.Notify(&notifications.NotifyRequest{
			User:    student.ID,
			Type:    notifications.InstantLessonAccept,
			Title:   "Tutor accepted instant session",
			Message: "Tutor accepted instant session",
			Action:  &action,
			Data:    map[string]interface{}{"lesson": lesson},
		})
	}

	return lesson, nil
}

func (s *sessions) Reject(u *store.UserMgo, i string) (*store.InstantSession, error) {
	lesson, err := store.InstantSessionsStore().ByID(bson.ObjectIdHex(i))
	if err != nil {
		return nil, errors.Wrap(err, "couldn't get instant session")
	}

	if lesson.Ended() {
		return lesson, errors.New("can't reject an already ended session")
	}

	if lesson.Status != store.InstantSessionCreated {
		return lesson, errors.New("can't reject a modified session")
	}

	student, ok := NewUsers().ByID(lesson.Student)
	if !ok {
		return nil, errors.New("couldn't get instant session student")
	}

	tutor, ok := NewUsers().ByID(lesson.Tutor)
	if !ok {
		return nil, errors.New("couldn't get instant session tutor")
	}

	var reason store.InstantSessionEndReason
	var action string

	switch u.ID.Hex() {
	case lesson.Tutor.Hex():
		// the tutor rejected the instant session request
		// announce the student about this
		reason = store.InstantSessionTutorReject

		message := fmt.Sprintf("%s is not available for your session. ", tutor.Profile.FirstName)
		message += "It's possible they stepped away from the computer for a moment."

		notifications.Notify(&notifications.NotifyRequest{
			User:    student.ID,
			Type:    notifications.InstantLessonReject,
			Title:   "Instant session was declined",
			Message: message,
			Action:  &action,
			Data:    map[string]interface{}{"lesson": lesson},
		})
	case lesson.Student.Hex():
		// the student cancelled the instant session request
		// announce the tutor about this
		reason = store.InstantSessionStudentReject

		message := fmt.Sprintf("%s cancelled the instant session request. ", student.Profile.FirstName)
		message += "It's possible they stepped away from the computer for a moment."

		notifications.Notify(&notifications.NotifyRequest{
			User:    tutor.ID,
			Type:    notifications.InstantLessonCancel,
			Title:   "Instant session was cancelled",
			Message: message,
			Action:  &action,
			Data:    map[string]interface{}{"lesson": lesson},
		})
	default:
		return nil, errors.New("invalid user rejecting request")
	}

	if _, err := lesson.SetStatus(store.InstantSessionRejected, reason); err != nil {
		return lesson, errors.Wrap(err, "couldn't set status to rejected")
	}

	return lesson, nil
}

func (s *sessions) Complete(i *store.InstantSession, u *store.UserMgo, room *Room) (err error) {
	reason := store.InstantSessionStudentQuit
	if i.Tutor.Hex() == u.ID.Hex() {
		reason = store.InstantSessionTutorQuit
	}

	i, err = i.SetStatus(store.InstantSessionCompleted)
	if err != nil {
		return errors.Wrap(err, "couldn't set instant session status to completed")
	}

	if err = i.SetEndReason(reason); err != nil {
		return errors.Wrap(err, "couldn't set instant session reason")
	}

	if i.EndedAt == nil || i.StartsAt == nil {
		return errors.New("end time or start time for instant session is nil")
	}

	room.Dto.Live = false
	if err := store.GetCollection("rooms").UpdateId(room.Dto.ID, bson.M{"$set": bson.M{"live": false}}); err != nil {
		return errors.Wrap(err, "couldn't update room to set live false")
	}

	student, ok := NewUsers().ByID(i.Student)
	if !ok {
		return errors.New("couldn't get student from instant session")
	}

	tutor, ok := NewUsers().ByID(i.Tutor)
	if !ok {
		return errors.New("couldn't get tutor from instant session")
	}

	// TODO: Bug for charging less than 50 cents can cause session to continue
	p := GetPayments()
	if err := p.ChargeForLesson(student, tutor, i.Duration().Minutes(), i.ID.Hex(), true); err != nil {
		return errors.Wrapf(err, "[!] couldn't auhtorize charge for student %s on instant lesson %v", student.Name(), i.ID.Hex())
	}

	if i.Duration() < 60*time.Minute {
		// only complete refer links if the instant session took more than 60 minutes
		return nil
	}

	referLinks, err := GetRefers().NeedPayment()
	if err != nil {
		return fmt.Errorf("couldn't get refer links: %s", err)
	}

	if len(referLinks) == 0 {
		return nil
	}

	transactionDetails := fmt.Sprintf("Transaction for instant session with ID %s", i.ID.Hex())

	lessonIntf := lessonInterface{
		ID:       i.ID,
		Tutor:    tutor,
		Student:  student,
		StartsAt: *i.StartsAt,
	}

	_, _, studentCost := lessonAmounts(float64(tutor.Tutoring.Rate), i.Duration().Minutes())
	amount := float64(studentCost) / 100

	for _, link := range referLinks {
		// check for students' link completion
		if i.Student.Hex() == (*link.Referral).Hex() {
			if err := completeLinkAndPay(link, amount, transactionDetails, lessonIntf); err != nil {
				// todo: error handling
			}
		}

		// check for tutor's link completion
		if i.Tutor.Hex() == (*link.Referral).Hex() {
			if err := completeLinkAndPay(link, amount, transactionDetails, lessonIntf); err != nil {
				// todo: error handling
			}
		}
	}

	return nil
}
*/
