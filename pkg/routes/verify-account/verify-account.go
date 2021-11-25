package verify_account

import (
	"net/http"

	"gitlab.com/learnt/api/pkg/checkr"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"

	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2/bson"
)

const (
	errUnauthorized uint = iota + 1
	errFormCompleted
	errFormInvalid
	errUserInvalid
	errCandidate
	errReport
	errInternal
)

type response struct {
	Error   bool   `json:"error"`
	Type    uint   `json:"type"`
	Message string `json:"message"`
	Raw     string `json:"raw"`
}

func requestHandler(c *gin.Context) {
	_, e := store.GetUser(c)
	if !e {
		return
	}

	id := c.Param("id")
	if !bson.IsObjectIdHex(id) {
		c.Status(http.StatusNotFound)
		return
	}

	userID := bson.ObjectIdHex(id)

	user, exist := services.NewUsers().ByID(userID)
	if !exist {
		c.JSON(http.StatusNotFound, response{
			Error:   false,
			Message: "user not found",
		})
		return
	}

	if user.CheckrData != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Type:    errFormCompleted,
			Message: "background check data already available",
		})
		return
	}

	candidateData := &checkr.Candidate{
		FirstName:    user.Profile.FirstName,
		LastName:     user.Profile.LastName,
		NoMiddleName: true,

		Phone:                user.Profile.Telephone,
		ZipCode:              user.Location.PostalCode,
		DateOfBirth:          user.Profile.Birthday.Format("2006-01-02"),
		SocialSecurityNumber: user.Profile.SocialSecurityNumber,
	}

	userEmail, err := user.MainEmail()
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Type:    errUserInvalid,
			Message: "user has no email address set",
			Raw:     err.Error(),
		})
	}

	candidateData.Email = userEmail

	api := checkr.New()
	candidate, err := api.CreateCandidate(candidateData)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Type:    errCandidate,
			Message: "couldn't create candidate",
			Raw:     err.Error(),
		})
		return
	}

	report, err := api.CreateReport(&checkr.Report{Package: checkr.PackageTaskerStd, CandidateID: candidate.ID})
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Type:    errReport,
			Message: "couldn't create report",
			Raw:     err.Error(),
		})
		return
	}

	users := services.NewUsers()
	err = users.SetCheckrData(user.ID, &store.UserCheckrData{CandidateID: candidate.ID, ReportID: report.ID})
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Type:    errInternal,
			Message: "couldn't set data to user",
			Raw:     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"candidate": candidate,
		"report":    report,
	})
}

// Setup adds all the routes to the router
func Setup(g *gin.RouterGroup) {
	g.PUT("request/:id", auth.Middleware, auth.IsAdminMiddleware, requestHandler)
}
