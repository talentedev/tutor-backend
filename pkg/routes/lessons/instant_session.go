package lessons

/*
type instantResponse struct {
	Lesson  *store.InstantSessionDTO `json:"lesson,omitempty"`
	Error   bool                     `json:"error,omitempty"`
	Message string                   `json:"message,omitempty"`
	Raw     interface{}              `json:"raw,omitempty"`
}

func getInstant(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "unauthorized"})
		return
	}

	lessonID := c.Param("lesson")
	if !bson.IsObjectIdHex(lessonID) {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "invalid lesson id"})
		return
	}

	lesson, err := services.InstantSessions().Get(lessonID)
	if err != nil {
		c.JSON(http.StatusNotFound, instantResponse{Error: true, Message: "lesson not found", Raw: err.Error()})
		return
	}

	if !lesson.HasParticipant(user.ID) {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "unauthorized"})
		return
	}

	dto, err := lesson.DTO()
	if err != nil {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "couldn't get lesson details", Raw: err.Error()})
		return
	}

	c.JSON(http.StatusOK, instantResponse{Lesson: dto})
}

func requestInstant(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "unauthorized"})
		return
	}

	req := services.InstantSessionRequest{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "invalid form", Raw: err})
		return
	}

	lesson, err := services.InstantSessions().New(user, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "couldn't create instant session", Raw: err.Error()})
		return
	}

	dto, err := lesson.DTO()
	if err != nil {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "couldn't get lesson details", Raw: err.Error()})
		return
	}

	c.JSON(http.StatusOK, instantResponse{Lesson: dto})
}

func acceptInstant(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "unauthorized"})
		return
	}

	lessonID := c.Param("lesson")
	if !bson.IsObjectIdHex(lessonID) {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "invalid lesson id"})
		return
	}

	lesson, err := services.InstantSessions().Accept(user, lessonID)
	if err != nil {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "couldn't accept instant session", Raw: err.Error()})
		return
	}

	dto, err := lesson.DTO()
	if err != nil {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "couldn't get lesson details", Raw: err.Error()})
		return
	}

	c.JSON(http.StatusOK, instantResponse{Lesson: dto})
}

func rejectInstant(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "unauthorized"})
		return
	}

	lessonID := c.Param("lesson")
	if !bson.IsObjectIdHex(lessonID) {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "invalid lesson id"})
		return
	}

	lesson, err := services.InstantSessions().Reject(user, lessonID)
	if err != nil {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "couldn't reject instant session", Raw: err.Error()})
		return
	}

	dto, err := lesson.DTO()
	if err != nil {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "couldn't get lesson details", Raw: err.Error()})
		return
	}

	c.JSON(http.StatusOK, instantResponse{Lesson: dto})
}

func listInstant(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "unauthorized"})
		return
	}

	ms := []bson.M{}

	if c.Query("from") != "" {
		from, err := time.Parse(time.RFC3339Nano, c.Query("from"))
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid time for start of period", Raw: err.Error()})
			return
		}
		ms = append(ms, bson.M{"ended_at": bson.M{"$gte": from}})
	}

	if c.Query("to") != "" {
		to, err := time.Parse(time.RFC3339Nano, c.Query("to"))
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Error: true, Message: "invalid time for end of period", Raw: err.Error()})
			return
		}
		ms = append(ms, bson.M{"starts_at": bson.M{"$lte": to}})
	}

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 100
	}

	offset, err := strconv.Atoi(c.Query("offset"))
	if err != nil {
		offset = 0
	}

	ms = append(ms, bson.M{"$or": []bson.M{
		{"tutor": user.ID},
		{"student": user.ID},
	}})

	query := services.InstantSessions().
		Find(bson.M{"$and": ms}).
		Sort("-starts_at").
		Skip(offset).Limit(limit)
	// store.PrintExplaination("instant session", query)

	var instants []store.InstantSession
	if err := query.All(&instants); err != nil {
		c.JSON(http.StatusBadRequest, instantResponse{Error: true, Message: "couldn't list instant sessions", Raw: err})
		return
	}

	c.JSON(http.StatusOK, store.InstantSessionsToDTO(instants))
}
*/
