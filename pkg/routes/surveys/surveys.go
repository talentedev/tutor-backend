package surveys

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/ws"
	"gopkg.in/mgo.v2/bson"
)

type FormRequest struct {
	Event string `json:"event"`
	Form  Form   `json:"form_json"`
}

type Form struct {
	Customer           FormCustomer `json:"customer"`
	Name               string       `json:"formName"`
	CompletedTimestamp *int64       `json:"completedTimestamp"`
	OpenedTimestamp    *int64       `json:"openedTimestamp"`
}

type FormCustomer struct {
	Email string `json:"email"`
}

func saveFormDetails(c *gin.Context) {
	req := FormRequest{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	logger.GetCtx(c).Infof("request %#v", req)

	query := bson.M{
		"username": req.Form.Customer.Email,
		"approval": store.ApprovalStatusApproved,
	}

	logger.GetCtx(c).Infof("mongo query %#v", query)

	var user *store.UserMgo
	if err := store.GetCollection("users").Find(query).One(&user); err != nil {
		c.JSON(http.StatusNotFound, core.NewErrorResponse("user not found"))
		return
	}

	openedAt := time.Now().UTC()
	if req.Form.OpenedTimestamp != nil {
		openedAt = millisToTime(*req.Form.OpenedTimestamp)
	}

	surveyDetails := store.SurveyDetails{
		OpenedAt: openedAt,
	}

	if req.Form.CompletedTimestamp != nil {
		completedAt := millisToTime(*req.Form.CompletedTimestamp)
		surveyDetails.CompletedAt = &completedAt
	}

	if user.Surveys == nil {
		surveys := map[string]store.SurveyDetails{
			req.Form.Name: surveyDetails,
		}

		user.Surveys = surveys
	} else {
		user.Surveys[req.Form.Name] = surveyDetails
	}

	err := store.GetCollection("users").UpdateId(
		user.ID,
		bson.M{
			"$set": bson.M{
				"surveys": user.Surveys,
			},
		},
	)

	if err != nil {
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse("could not save surveys to user"))
		return
	}

	if tc := ws.GetEngine().Hub.User(user.ID); tc != nil {
		_ = tc.Send(ws.Event{
			Type: req.Event,
			Data: ws.EventData{
				"form_name":    req.Form.Name,
				"opened_at":    req.Form.OpenedTimestamp,
				"completed_at": req.Form.CompletedTimestamp,
				"email":        req.Form.Customer.Email,
			},
		})
	}
}

func millisToTime(millis int64) time.Time {
	return time.Unix(0, millis*int64(time.Millisecond))
}

func Setup(g *gin.RouterGroup) {
	g.POST("/end-of-session", saveFormDetails)
}
