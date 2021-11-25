package checkr

import (
	"fmt"
	"net/http"
	"strconv"

	"gitlab.com/learnt/api/pkg/store"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/checkr"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"
	"gopkg.in/mgo.v2/bson"
)

type response struct {
	Error   bool   `json:"error"`
	Message string `json:"message"`
	Raw     string `json:"raw"`
}

func userGetHandler(c *gin.Context) {
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

	c.JSON(http.StatusOK, user.CheckrData)
}

func candidatesListHandler(c *gin.Context) {
	filters := new(checkr.CandidateFilters)
	params := new(checkr.PaginationParams)

	if c.Query("email") != "" {
		filters.Email = c.Query("email")
	}

	if c.Query("page") != "" {
		page, err := strconv.Atoi(c.Query("page"))
		if err == nil {
			params.Page = page
		}
	}

	if c.Query("per_page") != "" {
		perPage, err := strconv.Atoi(c.Query("per_page"))
		if err == nil {
			params.PerPage = perPage
		}
	}

	api := checkr.New()

	pagination, err := api.ListCandidates(filters, params)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Message: "couldn't list candidates",
			Raw:     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, pagination)
}

func candidatesCreateHandler(c *gin.Context) {
	var candidate *checkr.Candidate
	if err := c.BindJSON(&candidate); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Message: "invalid form submitted",
			Raw:     err.Error(),
		})
		return
	}

	api := checkr.New()
	newCandidate, err := api.CreateCandidate(candidate)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Message: "couldn't create candidate",
			Raw:     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, newCandidate)
}

func candidatesGetHandler(c *gin.Context) {
	api := checkr.New()
	candidate, err := api.RetrieveCandidate(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Error:   false,
			Message: "candidate not found",
			Raw:     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, candidate)
}

func (h *handler) reportsCreateHandler(c *gin.Context) {
	report := &checkr.Report{
		CandidateID: c.Query("candidate_id"),
		Package:     checkr.Package(c.Query("package")),
	}

	api := checkr.New()
	newReport, err := api.CreateReport(report)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Message: "couldn't create report",
			Raw:     err.Error(),
		})
		return
	}

	user, _ := h.Users.ByCandidateID(report.CandidateID)
	if user != nil {
		if err := user.UpdateApprovalStatus(store.ApprovalStatusBackgroundCheckRequested); err != nil {
			logger.GetCtx(c).Errorf("error updating approval status for user %s | cause: %#v", user.ID, err)
			c.JSON(http.StatusInternalServerError, "error updating user status")
			return
		}
	}

	c.JSON(http.StatusOK, newReport)
}

func (h *handler) invitationsCreateHandler(c *gin.Context) {
	invitation := &checkr.Invitation{
		CandidateID: c.Query("candidate_id"),
		Package:     checkr.Package(c.Query("package")),
	}

	api := checkr.New()
	newInvitation, err := api.CreateInvitation(invitation)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Error:   true,
			Message: "couldn't create invitation",
			Raw:     err.Error(),
		})
		return
	}

	user, _ := h.Users.ByCandidateID(invitation.CandidateID)
	if user != nil {
		if err := user.UpdateApprovalStatus(store.ApprovalStatusBackgroundCheckRequested); err != nil {
			logger.GetCtx(c).Errorf("error updating approval status for user %s | cause: %#v", user.ID, err)
			c.JSON(http.StatusInternalServerError, "error updating user status")
			return
		}
	}

	c.JSON(http.StatusOK, newInvitation)
}

func reportsGetHandler(c *gin.Context) {
	api := checkr.New()
	report, err := api.RetrieveReport(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Error:   false,
			Message: "report not found",
			Raw:     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, report)
}

func ssnTracesGetHandler(c *gin.Context) {
	api := checkr.New()
	ssnTrace, err := api.RetrieveSSNTrace(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Error:   false,
			Message: "ssn trace not found",
			Raw:     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, ssnTrace)
}

func sexOffenderSearchGetHandler(c *gin.Context) {
	api := checkr.New()
	sexOffenderSearch, err := api.RetrieveSexOffender(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Error:   false,
			Message: "sex offender search not found",
			Raw:     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, sexOffenderSearch)
}

func criminalSearchGetHandler(c *gin.Context) {
	api := checkr.New()
	criminalSearch, err := api.RetrieveCriminalSearch(c.Param("type"), c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Error:   false,
			Message: fmt.Sprintf("%s criminal search not found", c.Param("type")),
			Raw:     err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, criminalSearch)
}

// Setup adds all the routes to the router
func Setup(g *gin.RouterGroup) {
	// wire up dependencies for the handlers
	handler := &handler{
		Users: services.NewUsers(),
	}

	g.POST("/candidates", auth.Middleware, auth.IsAdminMiddleware, candidatesCreateHandler)
	g.POST("/reports", auth.Middleware, auth.IsAdminMiddleware, handler.reportsCreateHandler)
	g.POST("/invitations", auth.Middleware, auth.IsAdminMiddleware, handler.invitationsCreateHandler)

	g.GET("/user/:id", auth.Middleware, auth.IsAdminMiddleware, userGetHandler)

	g.GET("/candidates", auth.Middleware, auth.IsAdminMiddleware, candidatesListHandler)
	g.GET("/candidates/:id", auth.Middleware, auth.IsAdminMiddleware, candidatesGetHandler)

	g.GET("/reports/:id", auth.Middleware, auth.IsAdminMiddleware, reportsGetHandler)

	g.GET("/ssn_trace/:id", auth.Middleware, auth.IsAdminMiddleware, ssnTracesGetHandler)
	g.GET("/sex_offender_search/:id", auth.Middleware, auth.IsAdminMiddleware, sexOffenderSearchGetHandler)
	g.GET("/criminal_search/:type/:id", auth.Middleware, auth.IsAdminMiddleware, criminalSearchGetHandler)

	g.POST("/checkr_webhook", handler.checkrWebhookHandler)
}
