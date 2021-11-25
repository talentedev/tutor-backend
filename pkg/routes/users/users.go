package users

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/services/delivery"
	"gitlab.com/learnt/api/pkg/store"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/utils/timeline"
	"gitlab.com/learnt/api/pkg/ws"
	"gopkg.in/mgo.v2/bson"
)

type verifyRequest struct {
	Type string        `json:"type"`
	ID   bson.ObjectId `json:"id"`
}

type updateRequest struct {
	Profile             *store.Profile      `json:"profile,omitempty"`
	Location            *store.UserLocation `json:"location,omitempty"`
	Timezone            string              `json:"timezone,omitempty"`
	Disabled            *bool               `json:"disabled,omitempty"`
	Notes               []store.UserNote    `json:"notes,omitempty"`
	PromoteVideoAllowed *bool               `json:"promote_video_allowed,omitempty"`
	IsTestAccount       *bool               `json:"is_test_account,omitempty"`
	IsPrivate           *bool               `json:"is_private,omitempty"`
}

func verifyUser(c *gin.Context) {
	if !bson.IsObjectIdHex(c.Param("user")) {
		c.Status(http.StatusNotFound)
		return
	}

	userID := bson.ObjectIdHex(c.Param("user"))

	user, exist := services.NewUsers().ByID(userID)
	if !exist {
		c.JSON(
			http.StatusNotFound,
			core.NewErrorResponse(
				"User not found",
			),
		)

		return
	}

	req := verifyRequest{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	if err := services.NewUsers().Verify(user.ID, req.Type, req.ID); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}
}

func approveUser(c *gin.Context) {
	admin, e := store.GetUser(c)
	if !e {
		return
	}

	if !bson.IsObjectIdHex(c.Param("user")) {
		c.Status(http.StatusNotFound)

		return
	}

	userID := bson.ObjectIdHex(c.Param("user"))

	user, exist := services.NewUsers().ByID(userID)
	if !exist {
		c.JSON(
			http.StatusNotFound,
			core.NewErrorResponse(
				"User not found",
			),
		)

		return
	}

	if err := services.NewUsers().Approve(admin, userID); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	regToken, err := user.GetAuthenticationToken(store.AuthScopeCompleteAccount)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	approvedURL, err := core.AppURL("/start/apply?regtoken=%s", regToken.AccessToken)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}
	d := delivery.New(config.GetConfig())
	go d.Send(user, m.TPL_TUTOR_APPLICATION_APPROVED, &m.P{
		"URL_WITH_TOKEN": approvedURL,
	})
}

type createNoteRequest struct {
	Note string          `json:"note"`
	Type *store.NoteType `json:"type"`
}

func createNote(c *gin.Context) {
	admin, e := store.GetUser(c)
	if !e {
		return
	}

	if !bson.IsObjectIdHex(c.Param("user")) {
		c.Status(http.StatusNotFound)

		return
	}

	var request createNoteRequest
	if err := c.BindJSON(&request); err != nil {
		logger.GetCtx(c).Error(err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	userID := bson.ObjectIdHex(c.Param("user"))

	user, exist := services.NewUsers().ByID(userID)
	if !exist {
		c.JSON(
			http.StatusNotFound,
			core.NewErrorResponse(
				"User not found",
			),
		)

		return
	}

	note := store.UserNote{
		ID:        bson.NewObjectId(),
		Type:      request.Type,
		Note:      request.Note,
		CreatedAt: time.Now().UTC(),
		UpdatedBy: store.NoteUpdatedBy{
			ID:   admin.ID,
			Name: admin.GetName(),
		},
	}

	if err := user.CreateNote(note); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}
}

type rejectUserRequest struct {
	Reason string `json:"reason"`
}

func rejectUser(c *gin.Context) {
	admin, e := store.GetUser(c)
	if !e {
		return
	}

	if !bson.IsObjectIdHex(c.Param("user")) {
		c.Status(http.StatusNotFound)

		return
	}

	var request rejectUserRequest
	if err := c.BindJSON(&request); err != nil {
		logger.GetCtx(c).Error(err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	userID := bson.ObjectIdHex(c.Param("user"))

	user, exist := services.NewUsers().ByID(userID)
	if !exist {
		c.JSON(
			http.StatusNotFound,
			core.NewErrorResponse(
				"User not found",
			),
		)

		return
	}

	if err := services.NewUsers().Reject(admin, userID, request.Reason); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}
	d := delivery.New(config.GetConfig())
	go d.Send(user, m.TPL_TUTOR_APPLICATION_REJECTED, &m.P{"FIRST_NAME": user.GetFirstName()})
}

func getUsers(c *gin.Context) {
	c.JSON(http.StatusOK, services.NewUsers().All())
}

func getUserByID(sensitive bool) func(c *gin.Context) {
	return func(c *gin.Context) {
		if !bson.IsObjectIdHex(c.Param("user")) {
			c.Status(http.StatusNotFound)
			return
		}

		id := bson.ObjectIdHex(c.Param("user"))

		user, exist := services.NewUsers().ByID(id)
		if !exist {
			c.Status(http.StatusNotFound)
			return
		}

		c.JSON(http.StatusOK, user.Dto(sensitive))
	}
}

func getUnverifiedTutors(c *gin.Context) {
	auth.IsAdminMiddleware(c)

	users, _ := services.NewUsers().RequiresVerification()
	c.JSON(http.StatusOK, users)
}

func getPendingTutors(c *gin.Context) {
	auth.IsAdminMiddleware(c)

	users, _ := services.NewUsers().PendingTutors()
	c.JSON(http.StatusOK, users)
}

type createPasswordRequest struct {
	Password string `json:"password" binding:"required"`
}

func createPassword(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	r := createPasswordRequest{}

	if err := c.BindJSON(&r); err != nil {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	logger.GetCtx(c).Debugf("creating password %s for %s", r.Password, user.Username)

	if r.Password == "" {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"Password or token is empty",
			),
		)

		return
	}

	if err := user.UpdatePassword(r.Password, store.PasswordBcrypt); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	token, err := user.GetAuthenticationToken("auth")

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	c.JSON(http.StatusOK, token)
}

type resendEmailResponse struct {
	EmailSent bool   `json:"email_sent"`
	Error     bool   `json:"error"`
	Message   string `json:"message"`
}

type resendEmailRequest struct {
	ResendActivationEmail bool `json:"resend_activation_email" binding:"required"`
}

func resendActivationEmail(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	r := resendEmailRequest{}

	if err := c.BindJSON(&r); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	if !r.ResendActivationEmail {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"Password or token is empty",
			),
		)

		return
	}

	regToken, err := user.GetAuthenticationToken(store.AuthScopeResendActivationEmail)

	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	approvedURL, err := core.AppURL("/start/apply?regtoken=%s", regToken.AccessToken)
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	d := delivery.New(config.GetConfig())
	if err := d.Send(user, m.TPL_TUTOR_APPLICATION_APPROVED, &m.P{
		"URL_WITH_TOKEN": approvedURL,
	}); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)

		return
	}

	c.JSON(http.StatusOK, resendEmailResponse{
		EmailSent: true,
	})
}

func getLessons(c *gin.Context) {
	if !bson.IsObjectIdHex(c.Param("user")) {
		c.Status(http.StatusNotFound)
		return
	}

	id := bson.ObjectIdHex(c.Param("user"))

	user, exist := services.NewUsers().ByID(id)
	if !exist {
		c.Status(http.StatusNotFound)
		return
	}

	if !user.IsTutor() {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Specified user is not a tutor"))
		return
	}

	c.JSON(http.StatusOK, user.GetLessons(false))
}

type isAvailableResponse struct {
	Available bool `json:"available"`
}

// Check if user is available for a session within the specified time range
func isAvailable(c *gin.Context) {
	id := bson.ObjectIdHex(c.Param("user"))

	user, exist := services.NewUsers().ByID(id)
	if !exist {
		c.Status(http.StatusNotFound)
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")

	from, er := time.Parse(time.RFC3339Nano, fromStr)
	if er != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to parse from"))
		return
	}

	to, er := time.Parse(time.RFC3339Nano, toStr)
	if er != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to parse to"))
		return
	}

	from = from.In(time.UTC)
	to = to.In(time.UTC)

	// get lesson that conflicts session
	lesson := user.GetLessonBetween(from, to)

	if lesson != nil {
		c.JSON(http.StatusOK, isAvailableResponse{Available: false})
		return
	}

	// check if user is in a room
	rooms := services.VCRInstance().GetRooms()
	for _, room := range rooms {
		if room.UserConnected(*user) {
			c.JSON(http.StatusOK, isAvailableResponse{Available: false})
			return
		}
	}

	c.JSON(http.StatusOK, isAvailableResponse{Available: true})
}

func getAvailability(c *gin.Context) {
	if !bson.IsObjectIdHex(c.Param("user")) {
		c.Status(http.StatusNotFound)
		return
	}

	id := bson.ObjectIdHex(c.Param("user"))
	recurrent := c.Query("recurrent") == "true"

	user, exist := services.NewUsers().ByID(id)
	if !exist {
		c.Status(http.StatusNotFound)
		return
	}

	if !user.IsTutor() {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Specified user is not a tutor"))
		return
	}

	if unmodified, _ := c.GetQuery("unmodified"); unmodified != "" {
		c.JSON(http.StatusOK, user.Tutoring.Availability)
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")

	if fromStr != "" && toStr != "" {
		from, er := time.Parse(time.RFC3339, fromStr)
		if er != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to parse from"))
			return
		}

		to, er := time.Parse(time.RFC3339, toStr)
		if er != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to parse to"))
			return
		}

		blackout := user.GetBlackout(recurrent)

		var availability *timeline.Availability
		if blackout == nil {
			availability = user.GetAvailability(recurrent, false)
		} else {
			blackoutSlots, err := blackout.Get(from, to)
			if err != nil {
				c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to retrieve blackout:"+er.Error()))
				return
			}

			availability = user.GetAvailabilityWithBlackout(recurrent, blackoutSlots...)
		}

		if availability == nil {
			c.JSON(http.StatusNoContent, core.NewErrorResponse("No availability set"))
			return
		}

		avSlots, er := availability.Get(from, to)
		if er != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to retrieve availablitity:"+er.Error()))
			return
		}

		c.JSON(http.StatusOK, avSlots)

		return
	}

	c.JSON(http.StatusBadRequest, core.NewErrorResponse("No interval provided"))
}

func getBlackout(c *gin.Context) {
	if !bson.IsObjectIdHex(c.Param("user")) {
		c.Status(http.StatusNotFound)
		return
	}

	id := bson.ObjectIdHex(c.Param("user"))
	recurrent := c.Query("recurrent") == "true"

	user, exist := services.NewUsers().ByID(id)
	if !exist {
		c.Status(http.StatusNotFound)
		return
	}

	if !user.IsTutor() {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Specified user is not a tutor"))
		return
	}

	if unmodified, _ := c.GetQuery("unmodified"); unmodified != "" {
		c.JSON(http.StatusOK, user.Tutoring.Availability)
		return
	}

	fromStr := c.Query("from")
	toStr := c.Query("to")

	if fromStr != "" && toStr != "" {
		from, er := time.Parse(time.RFC3339, fromStr)
		if er != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to parse from"))
			return
		}

		to, er := time.Parse(time.RFC3339, toStr)
		if er != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to parse to"))
			return
		}

		av := user.GetBlackout(recurrent)

		if av == nil {
			c.JSON(http.StatusNoContent, core.NewErrorResponse("No availability set"))
			return
		}

		slots, er := av.Get(from, to)

		if er != nil {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to retrieve availablitity:"+er.Error()))
			return
		}

		c.JSON(http.StatusOK, slots)

		return
	}

	c.JSON(http.StatusBadRequest, core.NewErrorResponse("No interval provided"))
}

func getStudents(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.PrintStack(logger.ERROR, "recovered panic in getStudents %v", r)
			c.JSON(http.StatusInternalServerError, nil)
		}
	}()
	pageQ := c.DefaultQuery("page", "1")
	page, err := strconv.Atoi(pageQ)
	if err != nil {
		c.JSON(http.StatusBadRequest, nil)
		return
	}

	limitQ := c.DefaultQuery("limit", "50")
	limit, err := strconv.Atoi(limitQ)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}

	q := c.Query("q")

	students, count, err := services.NewUsers().GetStudents(page, limit, q)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	res := struct {
		Students []*store.UserDto `json:"students"`
		Count    int              `json:"count"`
	}{}
	studentsDto := make([]*store.UserDto, 0)
	for _, student := range students {
		studentsDto = append(studentsDto, student.Dto(true))
	}
	res.Students = studentsDto
	res.Count = count
	c.JSON(http.StatusOK, res)
	return
}

func getTutors(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.PrintStack(logger.ERROR, "receovered panic in getTutors: %v", r)
			c.JSON(http.StatusInternalServerError, nil)
		}
	}()
	page, err := strconv.Atoi(c.DefaultQuery("page", "1"))
	if err != nil {
		c.JSON(http.StatusBadRequest, nil)
		return
	}

	limit, err := strconv.Atoi(c.DefaultQuery("limit", "50"))
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}

	subjectId := c.DefaultQuery("subject", "")

	tutors, count, err := services.NewUsers().GetTutors(page, limit, c.Query("q"), c.Query("approvedOnly") == "true", subjectId)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	res := struct {
		Tutors []*store.UserDto `json:"tutors"`
		Count  int              `json:"count"`
	}{}
	tutorsDto := make([]*store.UserDto, 0)
	for _, tutor := range tutors {
		tutorsDto = append(tutorsDto, tutor.Dto(true))
	}
	res.Tutors = tutorsDto
	res.Count = count
	c.JSON(http.StatusOK, res)
}

type balanceResponse struct {
	Balance int64 `json:"balance"`
}

func getUserBalance(c *gin.Context) {
	id := c.Param("user")
	if !bson.IsObjectIdHex(id) {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("%v is invalid user id", id))
		return
	}

	user, exists := services.NewUsers().ByID(bson.ObjectIdHex(id))
	if !exists {
		c.JSON(http.StatusNotFound, nil)
		return
	}

	res := balanceResponse{}

	p := services.GetPayments()
	b, err := p.GetBalance(user)
	if err != nil {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("couldn't get user balance: %s", err))
		return
	}

	res.Balance = b

	c.JSON(http.StatusOK, res)
}

func getUserTransactions(c *gin.Context) {
	id := c.Param("user")
	if !bson.IsObjectIdHex(id) {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("%v is invalid user id", id))
		return
	}

	user, exists := services.NewUsers().ByID(bson.ObjectIdHex(id))
	if !exists {
		c.JSON(http.StatusNotFound, nil)
		return
	}

	from, err := time.Parse(time.RFC3339, c.Query("from"))
	if err != nil {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("%v is invalid date", from))
		return
	}
	to, err := time.Parse(time.RFC3339, c.Query("to"))
	if err != nil {
		c.JSON(http.StatusBadRequest, fmt.Sprintf("%v is invalid date", from))
		return
	}

	transactions := services.GetTransactions().GetTransactions(user, from, to)
	c.JSON(http.StatusOK, transactions)
}

func updateUser(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.PrintStack(logger.ERROR, "recovered panic in updateUser: %v", r)
			c.JSON(http.StatusBadRequest, "An error occured: "+fmt.Sprintf("%v", r))
			return
		}
	}()

	request := &updateRequest{}
	id := c.Param("user")
	if !bson.IsObjectIdHex(id) {
		c.JSON(http.StatusBadRequest, "Invalid id")
		return
	}

	if err := c.BindJSON(&request); err != nil {
		logger.GetCtx(c).Error(err)
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}

	users := services.NewUsers()
	user, exist := users.ByID(bson.ObjectIdHex(id))
	if !exist {
		c.JSON(http.StatusNotFound, "User not found")
		return
	}

	if request.Location != nil {
		if user.Location == nil {
			user.Location = &store.UserLocation{}
		}
		user.Location.Update(request.Location)
	}

	if request.Profile != nil {
		if err := user.Profile.Update(request.Profile); err != nil {
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}
	}

	if request.Timezone != "" {
		user.Timezone = request.Timezone
	}

	if request.Notes != nil {
		for _, note := range request.Notes {
			if note.ID == "" {
				note.ID = bson.NewObjectId()
			}

			user.Notes = append(user.Notes, note)
		}
	}

	if request.PromoteVideoAllowed != nil {
		user.Tutoring.PromoteVideoAllowed = *request.PromoteVideoAllowed
	}

	if request.Disabled != nil {
		user.Disabled = *request.Disabled
	}

	if isTestAccountUpdated(request, user) {
		auth.IsAdminMiddleware(c)
		user.IsTestAccount = *request.IsTestAccount
	}

	if request.IsPrivate != nil {
		user.Preferences.IsPrivate = *request.IsPrivate
	}

	if err := users.UpdateId(bson.ObjectIdHex(id), user); err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}

	c.JSON(http.StatusOK, user.Dto(true))
}

func isTestAccountUpdated(request *updateRequest, user *store.UserMgo) bool {
	return request.IsTestAccount != nil && *request.IsTestAccount != user.IsTestAccount
}

func getUserSessions(c *gin.Context) {
	defer func() {
		if r := recover(); r != nil {
			logger.PrintStack(logger.ERROR, "recovered panic in getUserSessions: %v", r)
			c.JSON(http.StatusBadRequest, "An error occured: "+fmt.Sprintf("%v", r))
			return
		}
	}()
	id := c.Param("user")
	if !bson.IsObjectIdHex(id) {
		c.JSON(http.StatusBadRequest, "Invalid id")
		return
	}
	fromString := c.Query("from")
	toString := c.Query("to")
	from, err := time.Parse(time.RFC3339, fromString)
	if err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}
	to, err := time.Parse(time.RFC3339, toString)
	if err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}
	rooms, err := services.GetLessons().GetCompletedLessonsForUser(bson.ObjectIdHex(id), from, to)
	if err != nil {
		c.JSON(http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, &rooms)
	return
}

// Setup adds users the routes to the router
func Setup(g *gin.RouterGroup) {
	ws.GetEngine().OnEnter(func(c *ws.Connection) {
		ws.GetEngine().Hub.NotifyOnlinePresence(c, store.Online)
	})

	ws.GetEngine().OnLeave(func(c *ws.Connection) {
		ws.GetEngine().Hub.NotifyOnlinePresence(c, store.Offline)
	})

	g.GET("", auth.Middleware, auth.IsAdminMiddleware, getUsers)

	// FOR ADMIN
	g.GET("/tutors/unverified", core.CORS, auth.Middleware, auth.IsAdminMiddleware, getUnverifiedTutors)
	g.GET("/tutors/pending", core.CORS, auth.Middleware, auth.IsAdminMiddleware, getPendingTutors)
	g.GET("/id/:user/sensitive", core.CORS, auth.Middleware, auth.IsAdminMiddleware, getUserByID(true))
	g.GET("/students", core.CORS, auth.Middleware, auth.IsAdminMiddleware, getStudents)
	g.GET("/tutors", core.CORS, auth.Middleware, auth.IsAdminMiddleware, getTutors)
	g.GET("/id/:user/balance", core.CORS, auth.Middleware, auth.IsAdminMiddleware, getUserBalance)
	g.GET("/id/:user/transactions", core.CORS, auth.Middleware, auth.IsAdminMiddleware, getUserTransactions)

	g.PUT("/:user", core.CORS, auth.Middleware, auth.IsAdminMiddleware, updateUser)
	g.POST("/id/:user/note", core.CORS, auth.Middleware, auth.IsAdminMiddleware, createNote)
	g.GET("/id/:user/sessions", getUserSessions)
	g.GET("/id/:user", getUserByID(false))

	g.GET("/id/:user/lessons", core.CORS, auth.Middleware, getLessons)
	g.GET("/id/:user/availability", core.CORS, auth.Middleware, getAvailability)
	g.GET("/id/:user/blackout", core.CORS, auth.Middleware, getBlackout)
	g.GET("/id/:user/availability/available", core.CORS, auth.Middleware, isAvailable)

	g.PUT("/:user/approve", core.CORS, auth.Middleware, auth.IsAdminMiddleware, approveUser)
	g.PUT("/:user/reject", core.CORS, auth.Middleware, auth.IsAdminMiddleware, rejectUser)
	g.PUT("/:user/verify", core.CORS, auth.Middleware, auth.IsAdminMiddleware, verifyUser)

	g.POST("/create-password", core.CORS, auth.Middleware, createPassword)
	g.POST("/resend-activation-email", core.CORS, auth.MiddlewareResend, resendActivationEmail)
}
