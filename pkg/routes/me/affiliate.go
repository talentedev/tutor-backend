package me

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/jinzhu/now"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/ics"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

const (
	statusCompleted = iota + 1
	statusConfirmed
)

type user struct {
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

/**
Payment status:
1 - done
2 - in dispute
3 - cancelled
*/

type payment struct {
	Status int     `json:"status"`
	Earned float64 `json:"earned"`
}

type lessonResponse struct {
	Tutor   user      `json:"tutor"`
	Student user      `json:"student"`
	Date    time.Time `json:"date"`
	Payment payment   `json:"payment"`
}

type lessonsResponse struct {
	Error   bool             `json:"error,omitempty"`
	Message string           `json:"message,omitempty"`
	Data    []lessonResponse `json:"data"`
}

func getUserData(id bson.ObjectId) (user, error) {
	var u store.UserMgo
	err := services.NewUsers().FindId(id).One(&u)
	if err != nil {
		return user{}, err
	}
	return user{Name: u.Name(), Avatar: u.Avatar()}, nil
}

func getPaymentData(id *bson.ObjectId) (payment, error) {
	var t *store.TransactionMgo
	err := store.GetCollection("transactions").FindId(id).One(&t)
	if err != nil {
		return payment{}, err
	}
	return payment{ /*Status: t.State, */ Earned: t.Amount}, nil
}

func getCalendarLessons(c *gin.Context) {
	user, exists := store.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponse("Unauthorized"))
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")

	tLayout := "2006-01-02T15:04:05Z"

	from, er := time.Parse(tLayout, fromStr)
	if er != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to parse from, format is:"+tLayout))
		return
	}

	to, er := time.Parse(tLayout, toStr)
	if er != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to parse to, format is:"+tLayout))
		return
	}

	from = from.In(time.UTC)
	to = to.In(time.UTC)

	c.JSON(http.StatusOK, user.GetLessonsSpread(from, to))
}

func getCalendarLessonsICS(c *gin.Context) {
	user, exists := store.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponse("Unauthorized"))
		return
	}

	lessons := services.NewUsers().GetLessons(user)

	name := fmt.Sprintf("calendar.ics")
	c.Header("Content-Type", "text/calendar; charset=utf-8")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", strings.ToLower(name)))
	c.Header("Content-Transfer-Encoding", "8bit")

	if err := ics.Lessons().Serve(c, user, lessons); err != nil {
		err = errors.Wrap(err, "failed to generate ICS file")
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))
	}

	return
}

func getCalendarLessonsDates(c *gin.Context) {
	user, exists := store.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponse("Unauthorized"))
		return
	}

	fromStr := c.Query("from")

	tLayout := "2006-01"

	from, er := time.Parse(tLayout, fromStr)
	if er != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to parse from, format is:"+tLayout))
		return
	}

	to := from.AddDate(0, 1, 0)

	from = from.In(time.UTC)
	to = to.In(time.UTC)

	logger.GetCtx(c).Debugf("to: %s | from: %s", to.Format(time.RFC3339), from.Format(time.RFC3339))

	c.JSON(http.StatusOK, user.GetLessonsDates(from, to))
}

func affiliateLessonsHandler(c *gin.Context) {
	user, exists := store.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponse("Unauthorized"))
		return
	}

	if user != nil && !user.IsAffiliate() {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponse("Unauthorized"))
		return
	}

	// get the sort type
	sortType, err := strconv.Atoi(c.Query("sort_type"))
	if err != nil {
		sortType = statusCompleted
	}

	if sortType != statusCompleted && sortType != statusConfirmed {
		sortType = statusCompleted
	}

	// get the from date, defaults to beginning of current month
	from, err := time.Parse(time.RFC3339Nano, c.Query("from"))
	if err != nil || from.IsZero() {
		from = now.BeginningOfMonth()
	}

	// get the to date, defaults to end of current month
	to, err := time.Parse(time.RFC3339Nano, c.Query("to"))
	if err != nil || to.IsZero() {
		to = now.EndOfMonth()
	}

	var res lessonsResponse
	res.Data = make([]lessonResponse, 0)

	var referLinks []store.ReferLink
	if err := services.GetRefers().Find(bson.M{"referrer": user.ID}).All(&referLinks); err != nil {
		res.Error = true
		res.Message = fmt.Sprintf("couldn't get refer links for user: %s", err)
		c.JSON(http.StatusBadRequest, res)
		return
	}

	var referrals []bson.ObjectId
	for _, link := range referLinks {
		referrals = append(referrals, *link.Referral)
	}

	sessions, err := getSessions(referrals, from, to)
	if err != nil {
		res.Error = true
		res.Message = err.Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	lessons, err := getLessons(referrals, from, to)
	if err != nil {
		res.Error = true
		res.Message = err.Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	res.Data = append(res.Data, sessions...)
	res.Data = append(res.Data, lessons...)

	c.JSON(http.StatusOK, res)
}

func getSessions(ids []bson.ObjectId, from, to time.Time) ([]lessonResponse, error) {
	var sessions []*store.LessonMgo
	err := store.GetCollection("lessons").Find(bson.M{
		"$or": []bson.M{
			{"tutor": bson.M{"$in": ids}},
			{"student": bson.M{"$in": ids}},
		},
		"instant":  true,
		"ended_at": bson.M{"$gte": from, "$lte": to},
	}).Sort("-ended_at").All(&sessions)

	if err != nil {
		return nil, fmt.Errorf("couldn't get instant sessions: %s", err)
	}

	response := make([]lessonResponse, 0)
	for _, session := range sessions {
		tutor, err := getUserData(session.Tutor)
		if err != nil {
			continue
		}

		if len(session.Students) == 0 {
			//TODO: ?
			panic("Not expected")
		}

		student, err := getUserData(session.Students[0])
		if err != nil {
			continue
		}

		response = append(response, lessonResponse{
			Tutor:   tutor,
			Student: student,
			Date:    *session.EndedAt,
			// todo: add taxes, fees, commissions
			Payment: payment{Status: 1, Earned: (15.0 / 100.0) * session.Duration().Minutes()},
		})
	}

	return response, nil
}

func getLessons(ids []bson.ObjectId, from, to time.Time) ([]lessonResponse, error) {
	var lessons []store.LessonMgo
	err := store.GetCollection("lessons").Find(bson.M{
		"$or": []bson.M{
			{"tutor": bson.M{"$in": ids}},
			{"student": bson.M{"$in": ids}},
		},
		"ends_at": bson.M{"$gte": from, "$lte": to},
	}).Sort("-ends_at").All(&lessons)

	if err != nil {
		return nil, fmt.Errorf("couldn't get lessons: %s", err)
	}

	response := make([]lessonResponse, 0)
	for _, lesson := range lessons {
		tutor, err := getUserData(lesson.Tutor)
		if err != nil {
			continue
		}

		tutorDto := lesson.TutorDto()
		if err != nil {
			continue
		}

		if len(lesson.Students) == 0 {
			continue
		}

		student, err := getUserData(lesson.Students[0])
		if err != nil {
			continue
		}

		amount := float64(tutorDto.Tutoring.Rate/60) * lesson.Duration().Minutes()

		var state int
		switch lesson.State {
		case store.LessonCancelled:
			state = 3
		default:
			state = 1
		}

		response = append(response, lessonResponse{
			Tutor:   tutor,
			Student: student,
			Date:    lesson.EndsAt,
			// todo: add taxes, fees, commissions
			Payment: payment{Status: state, Earned: amount},
		})
	}

	return response, nil
}
