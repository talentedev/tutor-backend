package lessons

import (
	"context"
	"fmt"
	"net/http"
	"time"
	"log"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/services/delivery"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/ws"
	"gopkg.in/mgo.v2/bson"
)

func setup(c *gin.Context) (*store.UserMgo, *store.LessonMgo, bool) {
	user, exist := store.GetUser(c)
	if !exist {
		return nil, nil, false
	}

	if !bson.IsObjectIdHex(c.Param("lesson")) {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid lesson id"})
		return nil, nil, false
	}

	id := bson.ObjectIdHex(c.Param("lesson"))

	lessonMgo, exist := store.GetLessonsStore().Get(id)
	if !exist {
		c.JSON(http.StatusNotFound, response{Error: true, Message: "lesson not found"})
		return nil, nil, false
	}

	if !lessonMgo.HasUser(user) {
		c.JSON(http.StatusUnauthorized, response{Error: true, Message: "unauthorized"})
		return nil, nil, false
	}

	return user, lessonMgo, true
}

type noteRequest struct {
	Note string `json:"note" binding:"required"`
}

func notePostHandler(c *gin.Context) {
	user, ok := store.GetUser(c)

	if !ok {
		c.JSON(http.StatusUnauthorized, response{Error: true, Message: "unauthorized"})
		return
	}

	var req noteRequest
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid form"})
		return
	}

	param := c.Param("lesson")
	if !bson.IsObjectIdHex(param) {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid lesson id"})
		return
	}

	lessonID := bson.ObjectIdHex(param)

	lesson, lessonExists := store.GetLessonsStore().Get(lessonID)

	if lessonExists {
		if !lesson.HasUser(user) {
			c.JSON(http.StatusUnauthorized, response{Error: true, Message: "unauthorized"})
			return
		}
	}

	note := &store.LessonNote{
		Note:   req.Note,
		Lesson: lessonID,
		User:   user.ID,
	}

	if err := note.Insert(); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't insert lesson note", Raw: err.Error()})
		return
	}

	utils.Bus().Emit(utils.EvLessonNoteCreated, note, user)

	otherParticipants, err := lesson.OtherParticipants(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Error: true, Message: "Failed to retrieve lesson other participants for sending email of note added", Raw: err.Error()})
		return
	}

	var lessonNoteBy = "0"

	var instantSession = "0"

	if user.ID.Hex() == lesson.Tutor.Hex() {
		lessonNoteBy = "1"
	}

	if lesson.IsInstantSession() {
		instantSession = "1"
	}

	lessonURL, err := core.AppURL("/main/account/calendar/details/%s", lesson.ID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Error: true, Message: err.Error(), Raw: err.Error()})
		return
	}

	tutorProfileURL, err := core.AppURL("/tutor/%s", lesson.Tutor.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Error: true, Message: err.Error(), Raw: err.Error()})
		return
	}

	lessonNotesURL, err := core.AppURL("/main/account/calendar/details/%s/notes?instant=false", lesson.ID.Hex())
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Error: true, Message: err.Error(), Raw: err.Error()})
		return
	}

	for _, participant := range otherParticipants {
		// TODO: Extract some of the params to lesson
		d := delivery.New(config.GetConfig())
		go d.Send(participant, m.TPL_LESSON_NOTE_CREATED, &m.P{
			"CREATED_BY":           user.GetFirstName(),
			"LESSON_INSTANT":       instantSession,
			"LESSON_SUBJECT":       lesson.FetchSubjectName(),
			"LESSON_URL":           lessonURL,
			"LESSON_NOTE_BY_TUTOR": lessonNoteBy,
			"TUTOR_PROFILE_URL":    tutorProfileURL,
			"LESSON_NOTES_URL":     lessonNotesURL,
			"STARTS_AT":            utils.FormatTime(lesson.StartsAt.In(participant.TimezoneLocation())),
			"ENDS_AT":              utils.FormatTime(lesson.EndsAt.In(participant.TimezoneLocation())),
			"NOTE":                 req.Note,
		})
	}

	c.JSON(http.StatusOK, note)
}

func noteListHandler(c *gin.Context) {
	_, ok := store.GetUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response{Error: true, Message: "unauthorized"})
		return
	}

	param := c.Param("lesson")
	if !bson.IsObjectIdHex(param) {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid lesson id"})
		return
	}

	lessonID := bson.ObjectIdHex(param)

	notes, err := store.LessonNotesStore().ByLesson(lessonID)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, Message: "couldn't get lesson notes", Raw: err.Error()})
		return
	}

	c.JSON(http.StatusOK, notes)
}

type InstantSessionRequest struct {
	Student *store.UserMgo `json:"student"`
	Tutor   *store.UserMgo `json:"tutor"`
	Subject *store.Subject `json:"subject"`
	Expire  time.Time
}

// InstantLessonRequests map[TutorID] = Request
var InstantLessonRequests = make(map[string]*InstantSessionRequest)

func onInstantSessionRequest(c *gin.Context) {
	if c.Param("lesson") != "instant" {
		return
	}

	user, ok := store.GetUser(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, response{Error: true, Message: "unauthorized"})
		return
	}

	// When tutor request this endpoint
	if existentRequest, exist := InstantLessonRequests[user.ID.Hex()]; exist {
		if c.Request.Method == "DELETE" {
			if tc := ws.GetEngine().Hub.User(existentRequest.Student.ID); tc != nil {
				_ = tc.Send(ws.Event{
					Type: "instant.deny",
					Data: ws.EventData{
						"tutor": existentRequest.Tutor.Dto(true),
					},
				})
			}

			delete(InstantLessonRequests, user.ID.Hex())

			return
		}

		delete(InstantLessonRequests, user.ID.Hex())

		if ws.GetEngine().Hub.User(existentRequest.Student.ID) == nil {
			c.JSON(http.StatusBadRequest, bson.M{"error": "Student no longer online"})
			return
		}

		err := services.GetLessons().CreateInstantSession(
			existentRequest.Student,
			existentRequest.Tutor,
			existentRequest.Subject,
		)

		if err != nil {
			c.JSON(http.StatusBadRequest, bson.M{"error": err.Error()})
			return
		}

		return
	}

	req := make(map[string]bson.ObjectId)
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, bson.M{"error": err.Error()})
		return
	}

	tutorID, tutorIDRequest := req["tutor"]
	if !tutorIDRequest {
		c.JSON(http.StatusBadRequest, bson.M{"error": "Param 'tutor' required"})
		return
	}

	subjectID, exist := req["subject"]
	if !exist {
		c.JSON(http.StatusBadRequest, bson.M{"error": "Param 'subject' required"})
		return
	}

	subject, exist := store.GetSubject(subjectID)
	if !exist {
		c.JSON(http.StatusBadRequest, bson.M{"error": "Subject does not exist"})
		return
	}

	tutor, tutorExist := services.NewUsers().ByID(tutorID)
	if !tutorExist || !tutor.IsTutor() {
		c.JSON(http.StatusBadRequest, bson.M{"error": "Tutor not found"})
		return
	}

	if request, exist := InstantLessonRequests[tutor.ID.Hex()]; exist {
		c.JSON(http.StatusNotAcceptable, bson.M{
			"error": fmt.Sprintf(
				"Already requested instant session for this tutor. Please wait %v seconds for response, then you can request instant session again.",
				int(time.Until(request.Expire).Round(time.Second).Seconds()),
			),
		})

		return
	}

	tmp := &InstantSessionRequest{
		Student: user,
		Tutor:   tutor,
		Subject: subject,
		Expire:  time.Now().Add(time.Minute * 2),
	}

	InstantLessonRequests[tutor.ID.Hex()] = tmp

	if tc := ws.GetEngine().Hub.User(tutor.ID); tc != nil {
		d := delivery.New(config.GetConfig())
		tutorProfileLink, err := core.AppURL("/main/account/calendar/details/%s", tutor.ID.Hex())
		if err != nil {
			log.Println(err.Error())
			c.JSON(http.StatusInternalServerError, response{Error: true, Message: err.Error(), Raw: err.Error()})
			return
		}
		go d.Send(tutor, m.TPL_INSTANT_LESSON_REQUEST, &m.P{
			"TUTOR_NAME": tutor.GetFirstName(),
			"STUDENT_NAME": user.GetFirstName(),
			"JOIN_SESSION": tutorProfileLink,
		})
		_ = tc.Send(ws.Event{
			Type: "instant.request",
			Data: ws.EventData{
				"student": user.Dto(true),
				"subject": subject,
				"timeout": time.Until(tmp.Expire).Seconds(),
			},
		})
	}
}

func onSocketUserEnter(c *ws.Connection) {
	user := c.GetUser()

	if !user.IsTutor() {
		return
	}

	if req, exist := InstantLessonRequests[user.ID.Hex()]; exist {
		_ = c.Send(ws.Event{
			Type: "instant.request",
			Data: ws.EventData{
				"student": req.Student.Dto(true),
				"subject": req.Subject,
				"timeout": time.Until(time.Now()).Seconds(),
			},
		})
	}
}

func instantSessionClean() {
	for {
		var nextExpire time.Time

		for id, req := range InstantLessonRequests {
			if time.Now().After(req.Expire) {
				if tc := ws.GetEngine().Hub.User(req.Student.ID); tc != nil {
					_ = tc.Send(ws.Event{Type: "instant.timeout"})
				}

				delete(InstantLessonRequests, id)
			}

			if nextExpire.IsZero() || req.Expire.Before(nextExpire) {
				nextExpire = req.Expire.Add(0)
			}
		}

		if nextExpire.IsZero() || nextExpire.Before(time.Now()) {
			//TODO: Is this performant?
			time.Sleep(time.Second)
		} else {
			wait := time.Until(nextExpire)
			logger.Get().Infof("Wait %s for checking instant lesson timeout\n", wait.String())
			time.Sleep(wait)
		}
	}
}

// SetupLessons  adds lessons routes to the router and starts the service
func SetupLessons(ctx context.Context, g *gin.RouterGroup) {
	go instantSessionClean()
	go services.GetLessons().Start(ctx)
	ws.GetEngine().OnEnter(onSocketUserEnter)

	openRoute := g.Group("", core.CORS)
	openRoute.GET("", get)

	authRequired := g.Group("", auth.Middleware, core.CORS)
	authRequired.POST("", create)

	// used :lesson as a hack. A moment will come to switch to chi
	authRequired.POST("/:lesson/instant", onInstantSessionRequest)
	authRequired.DELETE("/:lesson/instant", onInstantSessionRequest)

	authRequired.GET("/:lesson", getOne)
	authRequired.POST("/:lesson/reject", reject)
	authRequired.POST("/:lesson/accept", accept)
	authRequired.POST("/:lesson/complete", complete)

	authRequired.POST("/:lesson/propose", propose)
	authRequired.POST("/:lesson/propose/accept", proposeAccept)
	authRequired.POST("/:lesson/propose/decline", proposeDecline)

	authRequired.POST("/:lesson/recurrent", recurrentHandler)
	authRequired.POST("/:lesson/cancel", cancelHandler)

	authRequired.GET("/:lesson/notes", noteListHandler)
	authRequired.POST("/:lesson/notes", notePostHandler)
}
