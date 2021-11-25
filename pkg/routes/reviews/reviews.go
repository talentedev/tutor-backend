package reviews

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/utils/messaging/mail"
	"gopkg.in/mgo.v2/bson"
)

const (
	errInvalidUserID byte = iota
	errInvalidUser
	errNotATutor
	errAlreadyReviewed
	errInvalidForm
	errOnInsert
	errOnUpdate
	errReviewYourself
	errOnSendEmail
)

type reviewRequest struct {
	Communication   float64 `json:"communication" bson:"communication" binding:"required"`
	Clarity         float64 `json:"clarity" bson:"clarity" binding:"required"`
	Professionalism float64 `json:"professionalism" bson:"professionalism" binding:"required"`
	Patience        float64 `json:"patience" bson:"patience" binding:"required"`
	Helpfulness     float64 `json:"helpfulness" bson:"helpfulness" binding:"required"`
	Title           string  `json:"title" bson:"string" binding:"required"`
	Public          string  `json:"public" binding:"required"`
	Private         string  `json:"private"`
}

type response struct {
	Error     bool        `json:"error,omitempty"`
	ErrorType byte        `json:"error_type,omitempty"`
	Message   string      `json:"message,omitempty"`
	Raw       interface{} `json:"raw,omitempty"`
}

func getReviews(c *gin.Context) {
	if !bson.IsObjectIdHex(c.Param("user")) {
		c.JSON(http.StatusNotFound, response{Error: true, ErrorType: errInvalidUserID, Message: "invalid user id"})
		return
	}

	id := bson.ObjectIdHex(c.Param("user"))
	user, exist := services.NewUsers().ByID(id)
	if !exist {
		c.JSON(http.StatusNotFound, response{Error: true, ErrorType: errInvalidUser, Message: "user not found"})
		return
	}

	limit := 5
	offset := 0

	if c.Query("limit") != "" {
		if limitInt, err := strconv.Atoi(c.Query("limit")); err == nil {
			limit = limitInt
		}
	}

	if c.Query("offset") != "" {
		if offsetInt, err := strconv.Atoi(c.Query("offset")); err == nil {
			offset = offsetInt
		}
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"total":   user.CountReviews(),
		"average": user.AverageReviews(),
		"reviews": user.Reviews(limit, offset),
	})
}

func tutorReview(c *gin.Context) {
	reviewer, exists := store.GetUser(c)
	if !exists {
		return
	}

	if !bson.IsObjectIdHex(c.Param("user")) {
		c.JSON(http.StatusNotFound, response{Error: true, ErrorType: errInvalidUserID, Message: "invalid user id"})
		return
	}

	id := bson.ObjectIdHex(c.Param("user"))
	user, exist := services.NewUsers().ByID(id)
	if !exist {
		c.JSON(http.StatusNotFound, response{Error: true, ErrorType: errInvalidUser, Message: "user not found"})
		return
	}

	if !user.IsTutor() {
		c.JSON(http.StatusBadRequest, response{Error: true, ErrorType: errNotATutor, Message: "can only review tutors"})
		return
	}

	if user.ID.Hex() == reviewer.ID.Hex() {
		c.JSON(http.StatusBadRequest, response{Error: true, ErrorType: errReviewYourself, Message: "can't review yourself"})
		return
	}

	if _, yes := user.GetReviewFrom(reviewer); yes {
		c.JSON(http.StatusBadRequest, response{Error: true, ErrorType: errAlreadyReviewed, Message: "already reviewed this tutor"})
		return
	}

	r := reviewRequest{}

	if err := c.BindJSON(&r); err != nil {
		c.JSON(http.StatusBadRequest, response{Error: true, ErrorType: errInvalidForm, Message: "invalid form", Raw: err.Error()})
		return
	}

	review := &store.UserReview{
		ID:            bson.NewObjectId(),
		User:          user.ID,
		Reviewer:      reviewer.ID,
		Title:         r.Title,
		PublicReview:  r.Public,
		PrivateReview: r.Private,
		Approved:      true,
		Time:          time.Now(),

		Communication:   r.Communication,
		Clarity:         r.Clarity,
		Helpfulness:     r.Helpfulness,
		Patience:        r.Patience,
		Professionalism: r.Professionalism,
	}

	if err := mail.GetSender(config.GetConfig()).SendTo(m.HIRING_EMAIL, m.TPL_TUTOR_REVIEW, &m.P{
		"TUTOR_NAME":    user.Name(),
		"STUDENT_NAME":  reviewer.Name(),
		"PRIVATE_NOTES": r.Private,
	}); err != nil {
		c.JSON(http.StatusInternalServerError, response{Error: true, ErrorType: errOnSendEmail, Message: "couldn't send review email", Raw: err.Error()})
		return
	}

	if err := user.AddReview(review); err != nil {
		c.JSON(http.StatusInternalServerError, response{Error: true, ErrorType: errOnInsert, Message: "couldn't add review", Raw: err.Error()})
		return
	}

	if err := user.UpdateRating(); err != nil {
		c.JSON(http.StatusInternalServerError, response{Error: true, ErrorType: errOnUpdate, Message: "couldn't update rating", Raw: err.Error()})
		return
	}
}

func getMyReview(c *gin.Context) {
	reviewer, e := store.GetUser(c)
	if !e {
		return
	}

	id := bson.ObjectIdHex(c.Param("user"))
	user, exist := services.NewUsers().ByID(id)
	if !exist {
		c.JSON(http.StatusNotFound, response{Error: true, ErrorType: errInvalidUserID, Message: "invalid user id"})
		return
	}

	if !user.IsTutor() {
		c.JSON(http.StatusBadRequest, response{Error: true, ErrorType: errNotATutor, Message: "can only review tutors"})
		return
	}

	review, exist := user.GetReviewFrom(reviewer)

	if !exist {
		c.Status(http.StatusNotFound)
		return
	}

	c.JSON(http.StatusOK, review)
}

// Setup adds the reviews routes to the router
func Setup(g *gin.RouterGroup) {
	g.POST("/:user", auth.Middleware, tutorReview)
	g.GET("/:user", getReviews)
	g.GET("/:user/mine", auth.Middleware, getMyReview)
}
