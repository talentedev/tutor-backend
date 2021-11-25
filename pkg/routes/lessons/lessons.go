package lessons

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/notifications"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/services/delivery"
	"gitlab.com/learnt/api/pkg/store"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/utils/messaging/mail"
)

type response struct {
	Error   bool        `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
	Raw     interface{} `json:"raw,omitempty"`
}

type paginatedLessons struct {
	Lessons []*store.LessonDto `json:"lessons"`
	Length  int                `json:"length"`
}

type obscuredLesson struct {
	StartsAt  time.Time `json:"starts_at"`
	EndsAt    time.Time `json:"ends_at"`
	Recurrent bool      `json:"recurrent"`
}

type paginatedObscuredLessons struct {
	Lessons []*obscuredLesson `json:"lessons"`
	Length  int               `json:"length"`
}

func stateQuery(lte bool, state store.LessonState) bson.M {
	if lte {
		return bson.M{"state": bson.M{"$lte": state}}
	}

	return bson.M{"state": state}
}

func get(c *gin.Context) {
	// gets running lessons. does not accept other query params the get endpoint normally accepts
	findRunningLessons := c.Query("running")
	if findRunningLessons == "true" {
		paginated, err := getCurrentRunningLessons(c)
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't get lessons", Raw: err.Error()})
			return
		}

		lessonsDTO := store.LessonsToDTO(paginated.Lessons)
		c.JSON(http.StatusOK, paginatedLessons{lessonsDTO, paginated.Length})
		return
	}
	user, exists := store.GetUser(c)

	tutorID := c.Query("tutorId")
	if !exists && tutorID == "" {
		logger.GetCtx(c).Error("no user logged in and not looking for a specific tutor to see lessons")
		return
	}

	shouldObscureLessons := false
	if tutorID != "" {
		shouldObscureLessons = true

		if !bson.IsObjectIdHex(tutorID) {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "tutorId is not a valid id"})
		}
	}

	withID := c.Query("with")
	if withID != "" {
		if !bson.IsObjectIdHex(withID) {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid ID for other participant"})
			return
		}
	}

	var ms []bson.M

	if withID == "" {
		if tutorID == "" {
			m := bson.M{"$or": []bson.M{
				bson.M{"tutor": user.ID},
				bson.M{"students": user.ID},
			}}
			ms = append(ms, m)
		} else {
			ms = append(ms, bson.M{"tutor": tutorID})
		}
	} else {
		if tutorID == "" {
			m := bson.M{"$or": []bson.M{
				bson.M{"$and": []bson.M{
					bson.M{"tutor": user.ID},
					bson.M{"students": withID},
				}},
				bson.M{"$and": []bson.M{
					bson.M{"tutor": withID},
					bson.M{"students": user.ID},
				}},
			}}
			ms = append(ms, m)
		} else {
			m := bson.M{"$and": []bson.M{
				bson.M{"tutor": tutorID},
				bson.M{"students": withID},
			}}
			ms = append(ms, m)
		}
	}

	var lte bool
	if c.Query("lte") == "true" {
		lte = true
	}

	switch c.Query("state") {
	case "booked":
		ms = append(ms, stateQuery(lte, store.LessonBooked))
	case "confirmed":
		ms = append(ms, stateQuery(lte, store.LessonConfirmed))
	case "progress":
		ms = append(ms, stateQuery(lte, store.LessonProgress))
	case "completed":
		ms = append(ms, stateQuery(lte, store.LessonCompleted))
	case "cancelled":
		ms = append(ms, stateQuery(lte, store.LessonCancelled))
	case "all":
		ms = append(ms, stateQuery(true, store.LessonCancelled))
	default:
		ms = append(ms, stateQuery(true, store.LessonProgress))
	}

	var timeFilter []bson.M

	if c.Query("from") != "" {
		from, err := time.Parse(time.RFC3339Nano, c.Query("from"))
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid time for start of period", Raw: err.Error()})

			return
		}
		timeFilter = append(timeFilter, bson.M{"ends_at": bson.M{"$gte": from}})
	}

	if c.Query("to") != "" {
		to, err := time.Parse(time.RFC3339Nano, c.Query("to"))
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid time for end of period", Raw: err.Error()})
			return
		}
		timeFilter = append(timeFilter, bson.M{"starts_at": bson.M{"$lte": to}})
	}

	if len(timeFilter) > 0 {
		f := bson.M{"$or": []bson.M{
			bson.M{"recurrent": true},
			bson.M{"$and": append([]bson.M{bson.M{"recurrent": false}}, timeFilter...)},
		}}
		ms = append(ms, f)
	}

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 100
	}

	offset, err := strconv.Atoi(c.Query("offset"))
	if err != nil {
		offset = 0
	}

	paginated, err := store.GetLessonsStore().GetAllUserLessonsPaginated(ms, offset, limit)
	if err != nil {
		c.JSON(http.StatusBadGateway, response{Error: true, Message: "couldn't get lessons", Raw: err.Error()})
		return
	}

	if shouldObscureLessons {
		o := make([]*obscuredLesson, len(paginated.Lessons))
		for i, l := range paginated.Lessons {
			o[i] = &obscuredLesson{
				StartsAt:  l.StartsAt,
				EndsAt:    l.EndsAt,
				Recurrent: l.Recurrent,
			}
		}

		c.JSON(http.StatusOK, paginatedObscuredLessons{o, paginated.Length})

		return
	}

	lessonsDTO := store.LessonsToDTO(paginated.Lessons)
	c.JSON(http.StatusOK, paginatedLessons{lessonsDTO, paginated.Length})
}

func create(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	req := services.CreateLessonRequest{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	lessons, err := services.GetLessons().Create(user, &req)
	if err != nil {
		if lessonErr, ok := err.(*services.LessonErr); ok {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: lessonErr.Message, Raw: lessonErr})
		} else {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		}

		return
	}

	c.JSON(http.StatusOK, lessons)
}

type rejectRequest struct {
	Reason string `json:"reason" binding:"required"`
}

func reject(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	if !bson.IsObjectIdHex(c.Param("lesson")) {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Invalid lesson id"))
		return
	}

	id := bson.ObjectIdHex(c.Param("lesson"))

	lesson, exist := store.GetLessonsStore().Get(id)
	if !exist {
		c.JSON(http.StatusNotFound, core.NewErrorResponse("Invalid lesson id"))
		return
	}

	req := rejectRequest{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Invalid lesson id"))
		return
	}

	err := services.GetLessons().Reject(user, lesson, req.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))
		return
	}
}

func accept(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	if !bson.IsObjectIdHex(c.Param("lesson")) {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Invalid lesson id"))
		return
	}

	id := bson.ObjectIdHex(c.Param("lesson"))

	lesson, exist := store.GetLessonsStore().Get(id)
	if !exist {
		c.JSON(http.StatusNotFound, core.NewErrorResponse("Lesson not found"))
		return
	}

	err := services.GetLessons().Accept(lesson, user)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}
}

type proposeRequest struct {
	Subject string `json:"subject"`

	Meet     int    `json:"meet"`
	Location string `json:"location"`

	When time.Time `json:"when"`
	Ends time.Time `json:"ends"`

	Recurrent      bool `json:"recurrent"`
	RecurrentCount int  `json:"recurrent_count"`
}

func propose(c *gin.Context) {
	user, lesson, ok := setup(c)
	if !ok {
		return
	}

	req := proposeRequest{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid form"})
		return
	}

	if !lesson.CanBeModified() {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "lesson can't be modified"})
		return
	}

	if !lesson.HasUser(user) {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "unauthorized"})
		return
	}

	tutor, exist := services.NewUsers().ByID(lesson.Tutor)
	if !exist {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't get tutor details"})
		return
	}

	if lesson.Tutor.Hex() == tutor.ID.Hex() {
		if !user.IsFreeIgnoreIDs(req.When, lesson.Duration(), []bson.ObjectId{lesson.ID}) {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "student isn't free"})
			return
		}
	} else {
		if !tutor.IsFreeIgnoreIDs(req.When, lesson.Duration(), []bson.ObjectId{lesson.ID}) {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "tutor isn't free"})
			return
		}
	}

	for _, studentID := range lesson.Students {
		student, exist := services.NewUsers().ByID(studentID)
		if !exist {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't get student details"})
			return
		}

		if !student.IsFreeIgnoreIDs(req.When, lesson.Duration(), []bson.ObjectId{lesson.ID}) {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "student isn't free"})
			return
		}
	}

	var change store.LessonChangeProposal

	var propose bool

	if req.Subject != "" {
		if !bson.IsObjectIdHex(req.Subject) {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid subject"})
			return
		}

		subject := bson.ObjectIdHex(req.Subject)

		var hasSubject bool

		for _, s := range tutor.Tutoring.Subjects {
			if s.Subject.Hex() == subject.Hex() {
				hasSubject = true
				break
			}
		}

		if !hasSubject {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "tutor doesn't have that subject"})
			return
		}

		propose = true
		change.Subject = subject
	} else {
		change.Subject = lesson.Subject
	}

	if req.Meet != 0 {
		meet := store.Meet(req.Meet)

		switch meet {
		case store.MeetOnline:
			req.Location = "" // meeting online doesn't need an address
		case store.MeetInPerson:
			if req.Location == "" {
				c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid location"})
				return
			}
		default:
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid meeting place"})
			return
		}

		propose = true
		change.Meet = meet
		change.Location = req.Location
	} else {
		change.Meet = lesson.Meet
		change.Location = lesson.Location
	}

	if !req.When.IsZero() && !req.Ends.IsZero() {
		propose = true
		change.StartsAt = req.When
		change.EndsAt = req.Ends
	}

	if !propose {
		return
	}

	change.User = user.ID

	change.CreatedAt = time.Now()
	if err := services.GetLessons().ProposeChange(lesson, user, change); err != nil {
		if lessonErr, ok := err.(*services.LessonErr); ok {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: lessonErr.Message, Raw: lessonErr})
		} else {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't propose lesson changes", Raw: err.Error()})
		}

		return
	}

	// notify other participant
	otherParticipants, err := lesson.OtherParticipants(user.ID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't get other participant", Raw: err.Error()})
		return
	}
	if len(otherParticipants) < 1 {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "other participant not found"})
		return
	}
	if err := sendProposalMessageNotification(c, user, otherParticipants[0], lesson); err != nil {
		logger.GetCtx(c).Errorf("couldn't send message notification: %v", err)
	}

	if err := lesson.SetState(store.LessonBooked, nil); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't change lesson status to booked"})
		return
	}

	if err := lesson.SetRecurrent(req.Recurrent, req.RecurrentCount); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: fmt.Sprintf("couldn't set recurring lesson value to %d", req.RecurrentCount), Raw: err})
		return
	}
}

type messageData struct {
	Type    byte        `json:"type"`
	Title   string      `json:"title"`
	Content string      `json:"content"`
	Data    interface{} `json:"data"`
}

type notificationRequest struct {
	From string      `json:"from"`
	To   string      `json:"to"`
	Data messageData `json:"data"`
}

func sendProposalMessageNotification(c *gin.Context, from, to *store.UserMgo, lesson *store.LessonMgo) error {
	token, ok := c.Get("token")
	if !ok {
		return fmt.Errorf("token missing from request context")
	}

	endpoint := fmt.Sprint(config.GetConfig().GetString("messenger.address"), "/notifications")
	updateURL, _ := url.Parse(endpoint)

	params := url.Values{}
	params.Set("access_token", token.(string))
	updateURL.RawQuery = params.Encode()

	title := "A new lesson change was proposed to your lesson"
	content := ""
	d := notificationRequest{
		From: from.ID.Hex(),
		To:   to.ID.Hex(),
		Data: messageData{
			Type:    1, // DataTypeReschedule MessageDataType = iota + 1
			Title:   title,
			Content: content,
			Data:    map[string]string{"lesson_id": lesson.ID.Hex()},
		},
	}

	data, err := json.Marshal(d)
	if err != nil {
		return fmt.Errorf("couldn't marshal data into json: %s", err)
	}

	res, err := http.Post(updateURL.String(), "application/json", bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("failed to request %s: %s", updateURL.String(), err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("received non ok status code: %d", res.StatusCode)
	}

	return nil
}

func proposeAccept(c *gin.Context) {
	user, lesson, ok := setup(c)
	if !ok {
		return
	}

	if !lesson.CanBeModified() {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "lesson can't be modified"})
		return
	}

	if !lesson.HasUser(user) {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "unauthorized"})
		return
	}

	if lesson.EveryoneAccepted() {
		c.JSON(http.StatusOK, response{Message: "no change request to accept"})
		return
	}

	everyone, err := lesson.AcceptBy(user)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't accept change request", Raw: err.Error()})
		return
	}

	if !everyone {
		c.JSON(http.StatusOK, response{Message: "accepted change request"})
		return
	}

	all := false
	if c.Query("all") != "" {
		all, err = strconv.ParseBool(c.Query("all"))
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "query param(s) invalid", Raw: err})
			return
		}
	}

	if err := lesson.SetState(store.LessonConfirmed, &all); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't set lesson as confirmed", Raw: err.Error()})
		return
	}

	change := lesson.ChangeProposals[len(lesson.ChangeProposals)-1]
	if _, err := change.Apply(lesson.ID); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't apply changes to lesson", Raw: err.Error()})
		return
	}

	title := fmt.Sprintf("Your lesson change was approved by %s", user.Name())
	message := fmt.Sprintf("The lesson on %s has the latest change approved.", lesson.FetchSubjectName())
	_ = services.GetLessons().NotifyExcept(lesson, user, notifications.LessonChangeRequestAccepted, title, message)

	c.Status(http.StatusOK)
}

func proposeDecline(c *gin.Context) {
	user, lesson, ok := setup(c)
	if !ok {
		return
	}

	if !lesson.CanBeModified() {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "lesson can't be modified"})
		return
	}

	if !lesson.HasUser(user) {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "unauthorized"})
		return
	}

	if lesson.EveryoneAccepted() {
		c.JSON(http.StatusOK, response{Message: "no change request to decline"})
		return
	}

	// cancelling the lesson

	if lesson.State == store.LessonCancelled {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "can't modify cancelled lesson"})
		return
	}

	role := "student"
	if lesson.Tutor.Hex() == user.ID.Hex() {
		role = "tutor"
	}

	var err error

	all := false
	if c.Query("all") != "" {
		all, err = strconv.ParseBool(c.Query("all"))
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "query param(s) invalid", Raw: err})
			return
		}
	}

	if err := lesson.SetState(store.LessonCancelled, &all, map[string]interface{}{
		"user":   user.Name(),
		"reason": fmt.Sprintf("Lesson was cancelled by %s %s for declining the change proposal.", role, user.Name()),
	}); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't set cancelled state to lesson", Raw: err})
		return
	}

	title := fmt.Sprintf("Your lesson change was not approved by %s", user.Name())
	message := fmt.Sprintf("The lesson on %s was cancelled.", lesson.FetchSubjectName())
	_ = services.GetLessons().NotifyExcept(lesson, user, notifications.LessonChangeRequestDeclined, title, message)

	title = "Lesson cancelled"
	message = fmt.Sprintf("Lesson was cancelled by %s %s.", role, user.Name())
	services.GetLessons().NotifyAll(lesson, notifications.LessonSystemCancelled, title, message)

	c.Status(http.StatusOK)
}

func getOne(c *gin.Context) {
	_, lesson, exist := setup(c)
	if !exist {
		return
	}

	dto, err := lesson.DTO()
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't get lesson", Raw: err.Error()})
		return
	}

	c.JSON(http.StatusOK, dto)
}

func getCurrentRunningLessons(c *gin.Context) (*store.PaginatedLessons, error) {
	// only allow admin to access
	if !auth.IsAdmin(c) {
		return nil, errors.New("unauthorized")
	}
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 10000
	}

	offset, err := strconv.Atoi(c.Query("offset"))
	if err != nil {
		offset = 0
	}

	return services.GetLessons().GetCurrentRunningLessons(offset, limit)
}

func complete(c *gin.Context) {
	user, lesson, ok := setup(c)
	if !ok {
		return
	}

	if !lesson.HasUser(user) || lesson.Tutor.Hex() != user.ID.Hex() {
		c.JSON(http.StatusNotFound, core.NewErrorResponse("Only tutor can complete the lesson"))
		return
	}

	if err := services.GetLessons().Complete(lesson); err != nil {
		c.JSON(http.StatusNotFound, core.NewErrorResponse(err.Error()))
		return
	}
}

type recurrentForm struct {
	Recurrent      bool `json:"recurrent" binding:"required"`
	RecurrentCount int  `json:"recurrent_count" binding:"required"`
}

func recurrentHandler(c *gin.Context) {
	_, lesson, ok := setup(c)
	if !ok {
		return
	}

	if lesson.State == store.LessonCancelled {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "can't modify cancelled lesson"})
		return
	}

	var f recurrentForm
	if err := c.BindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't bind form", Raw: err})
		return
	}

	if err := lesson.SetRecurrent(f.Recurrent, f.RecurrentCount); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't unset recurrent lesson", Raw: err})
		return
	}
}

type cancelForm struct {
	Reason string `json:"reason" binding:"required"`
}

func cancelHandler(c *gin.Context) {
	user, lesson, ok := setup(c)
	if !ok {
		return
	}

	if lesson.State == store.LessonCancelled {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "can't modify cancelled lesson"})
		return
	}

	role := "student"
	if lesson.Tutor.Hex() == user.ID.Hex() {
		role = "tutor"
	}

	var f cancelForm
	if err := c.BindJSON(&f); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't bind form", Raw: err})
		return
	}

	all := false

	var err error
	if c.Query("all") != "" {
		all, err = strconv.ParseBool(c.Query("all"))
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "query param(s) invalid", Raw: err})
			return
		}
	}

	if err := lesson.SetState(store.LessonCancelled, &all, map[string]interface{}{
		"user":   user.Name(),
		"reason": fmt.Sprintf("Lesson was cancelled by %s %s using reason %q.", role, user.Name(), f.Reason),
	}); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't set cancelled state to lesson", Raw: err})
		return
	}

	title := "Lesson cancelled"
	message := fmt.Sprintf("Lesson was cancelled by %s %s.", role, user.Name())
	services.GetLessons().NotifyAll(lesson, notifications.LessonSystemCancelled, title, message)
	data := m.P{
		"SUBJECT":  title,
		"DAY_TIME": lesson.WhenFormatTime(""),
	}

	conf := config.GetConfig()
	d := delivery.New(conf)
	if lesson.Tutor.Hex() != user.ID.Hex() {
		// student
		data["TUTOR_NAME"] = lesson.GetTutor().Name()
		data["STUDENT_NAME"] = user.Name()
		if lesson.StartsWithin24Hours() {
			services.GetLessons().AuthorizeCharge(user, lesson)

			if err = d.Send(lesson.GetTutor(), m.TPL_CANCELLED_WITHIN_24_HOURS, &data); err == nil {
				err = mail.GetSender(conf).SendTo(m.HIRING_EMAIL, m.TPL_CANCELLED_WITHIN_24_HOURS, &data)
			}
		} else {
			err = d.Send(lesson.GetTutor(), m.TPL_CANCELLED_GREATER_THAN_24_HOURS, &data)
		}
	} else if lesson.Tutor.Hex() == user.ID.Hex() {
		// tutor
		// notify admin
		tpl := m.TPL_TUTOR_CANCELLED
		data["TUTOR_NAME"] = user.Name()
		if lesson.StartsWithin24Hours() {
			tpl = m.TPL_TUTOR_CANCELLED_WITHIN_24_HOURS
		}

		for _, studentID := range lesson.Students {
			if student, ok := services.NewUsers().ByID(studentID); ok {
				data["STUDENT_NAME"] = student.Name()
				if err = mail.GetSender(conf).SendTo(m.HIRING_EMAIL, tpl, &data); err != nil {
					break
				}
			}
		}
	}

	if err != nil {
		c.JSON(http.StatusOK, response{Error: true, Message: "couldn't send cancellation email", Raw: err})
		return
	}

	c.Status(http.StatusOK)
}
