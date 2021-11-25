package messenger

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
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

// The event types
const (
	MsgEventAuth         string = "auth"
	MsgEventAuthError    string = "auth.error"
	MsgEventAuthSuccess  string = "auth.success"
	MsgEventMessage      string = "message"
	MsgEventMessageError string = "message.error"

	MsgEventThreadCreated  string = "thread.created"
	MsgEventMessageCreated string = "message.created"

	MsgEventUserPresence string = "user.presence"
)

type handler struct {
	mailer string
}

type (
	// CreateThreadRequest HTTP body for creating threads
	CreateThreadRequest struct {
		Name    string            `json:"name"`
		Message *store.Message    `json:"message"`
		Tutor   bson.ObjectId     `json:"tutor" binding:"required"`
		Meta    *store.ThreadMeta `json:"meta,omitempty"`
	}
	//flagRequest is the body used to make a request to flag a thread
	flagRequest struct {
		Reason string `json:"reason"`
	}
)

func httpError(c *gin.Context, err error) {
	c.JSON(
		http.StatusBadRequest,
		core.NewErrorResponse(
			err.Error(),
		),
	)
}

// CreateThread creats a new message thread
func CreateThread(creator store.ThreadParticipant, participants []store.ThreadParticipant, meta *store.ThreadMeta) (*store.ThreadDto, error) {
	var participantsIds = make([]bson.ObjectId, len(participants))
	for i, participant := range participants {
		participantsIds[i] = participant.ID
	}

	users := services.NewUsers().ByIDs(participantsIds)

	if len(users) != len(participants) {
		return nil, errors.New("One of the participants missing from system")
	}

	createThread := &store.Thread{
		Creator:      creator.ID,
		Participants: participantsIds,
		Meta:         meta,
		MessageTime:  time.Now(), // allow creating thread with no message to appear on top
	}

	thread, err := store.GetThreads().Create(createThread)
	if err != nil {
		return nil, err
	}

	return thread, nil
}

func getOrCreateThread(user store.ThreadParticipant, participants []store.ThreadParticipant, meta *store.ThreadMeta) (*store.ThreadDto, error) {
	var thread *store.ThreadDto
	var err error
	thread, err = store.GetThreads().ParticipantsThread(participants)
	if err != nil {
		thread, err = CreateThread(user, participants, meta)
		if err != nil {
			return nil, err
		}
	}

	return thread, nil
}

func getRequestThread(user *store.UserMgo, threadID string) (*store.ThreadDto, error) {

	if !bson.IsObjectIdHex(threadID) {
		return nil, errors.New("invalid thread ID")
	}

	thread, err := store.GetThreads().WithID(bson.ObjectIdHex(threadID))
	if err != nil {
		return nil, err
	}

	if !user.IsAdmin() && !thread.HasParticipant(*user) {
		return nil, errors.New("user not participant of thread")
	}

	if user.IsAdmin() {
		// add to participanst
	}

	return thread, nil
}

func createThread(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}
	req := CreateThreadRequest{}
	if err := c.BindJSON(&req); err != nil {
		httpError(c, err)
		return
	}

	tutor, tutorExists := services.NewUsers().ByID(req.Tutor)

	if !tutorExists {
		httpError(c, errors.New("Tutor not found"))
		return
	}

	var participants = []store.ThreadParticipant{
		*user.ToThreadParticipant(),
		*tutor.ToThreadParticipant(),
	}
	var thread *store.ThreadDto
	var err error

	if req.Meta == nil {
		thread, err = getOrCreateThread(*user.ToThreadParticipant(), participants, req.Meta)
		if err != nil {
			httpError(c, err)
			return
		}
	} else {
		switch (*req.Meta)["instant"].(type) {
		case bool:
			isInstant, ok := (*req.Meta)["instant"].(bool)
			if ok && isInstant {
				thread, err = getOrCreateThread(*user.ToThreadParticipant(), participants, req.Meta)
				if err != nil {
					httpError(c, err)
					return
				}
			}
		}
	}

	if thread == nil {
		thread, err = CreateThread(*user.ToThreadParticipant(), participants, req.Meta)
		if err != nil {
			httpError(c, err)
			return
		}
	}

	if req.Message == nil {
		httpError(c, errors.New("message can't be nil"))
		return
	}

	message := store.GetMessages().NewMessage(user, thread.ID, req.Message.Type, req.Message.Body, nil)

	if err := thread.AddMessage(message); err != nil {
		httpError(c, err)
		return
	}

	go ws.GetEngine().Notify(
		MsgEventThreadCreated,
		thread.ParticipantsIDS(),
		ws.EventData{
			"thread":  thread,
			"message": &thread.LastMessage,
		},
	)

	d := delivery.New(config.GetConfig())
	for _, id := range thread.ParticipantsIDS() {
		if user.ID.Hex() != id {
			if other, ok := services.NewUsers().ByID(bson.ObjectIdHex(id)); ok {
				chatURL, err := core.AppURL(fmt.Sprintf("/main/inbox/%s", thread.ID.Hex()))
				if err != nil {
					httpError(c, err)
				}

				go d.Send(other, m.TPL_MESSAGE_NOTIFICATION, &m.P{
					"FIRST_NAME": other.GetFirstName(),
					"OTHER_NAME": user.GetFirstName(),
					"CHAT_URL":   chatURL,
				})

			}
		}
	}

	c.JSON(http.StatusCreated, thread)
}

func extractIDs(users []store.ThreadParticipant) (ids []string) {
	ids = make([]string, 0)
	for _, u := range users {
		ids = append(ids, u.ID.Hex())
	}
	return ids
}

func routeDeleteThread(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	thread, err := getRequestThread(user, c.Param("id"))
	if err != nil {
		httpError(c, err)
		return
	}

	if err := thread.DeleteFor(*user); err != nil {
		httpError(c, err)
		return
	}

	c.Status(http.StatusGone)
}

func getThreads(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	threads, err := store.GetThreads().Fetch(*user, c.Query("archived") == "true")
	if err != nil {
		httpError(c, err)
		return
	}
	c.JSON(http.StatusOK, threads)
}

func routeArchiveThread(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	thread, err := getRequestThread(user, c.Param("id"))
	if err != nil {
		httpError(c, err)
		return
	}

	if err := thread.ArchiveFor(*user); err != nil {
		httpError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

func routeGetThreadMessages(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	skipS := c.Query("skip")
	limitS := c.Query("limit")

	var skip, limit int64 = 0, 15

	if skipS != "" {
		skipN, err := strconv.ParseInt(skipS, 10, 64)
		if err != nil {
			httpError(c, errors.New("invalid skip value"))
			return
		}
		skip = skipN
	}

	if limitS != "" {
		limitN, err := strconv.ParseInt(limitS, 10, 64)
		if err != nil {
			httpError(c, err)
			return
		}
		limit = limitN
	}

	thread, err := getRequestThread(user, c.Param("id"))
	if err != nil {
		httpError(c, err)
		return
	}

	model := store.GetMessages()
	messages := model.ForThread(thread, skip, limit)
	total := model.CountForThread(thread)

	c.JSON(200, bson.M{
		"messages": messages,
		"total":    total,
	})
}

func getThread(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	thread, err := getRequestThread(user, c.Param("id"))
	if err != nil {
		httpError(c, err)
		return
	}

	c.JSON(http.StatusOK, thread)
}

func flagThread(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	thread, err := getRequestThread(user, c.Param("id"))
	if err != nil {
		httpError(c, err)
		return
	}

	fr := flagRequest{}
	if err := c.BindJSON(&fr); err != nil {
		httpError(c, errors.Wrap(err, "cannot bind JSON"))
		return
	}

	tf, err := thread.Flag(*user, fr.Reason)
	if err != nil {
		httpError(c, err)
		return
	}

	// Get admins and send message
	root, exist := services.NewUsers().ByRole(store.RoleRoot)
	if !exist {
		logger.GetCtx(c).Error("no root user for sending flagged message")
		return
	}
	d := delivery.New(config.GetConfig())
	d.Send(root, m.TPL_FLAG_MESSAGE, &m.P{
		"THREAD": thread.ID.Hex(),
		"REASON": fr.Reason,
	})

	c.JSON(http.StatusCreated, tf)
}

// CreateMessageRequest is the HTTP body for creating message
type CreateMessageRequest struct {
	Thread  string        `json:"thread" binding:"required"`
	Message store.Message `json:"message" binding:"required"`
}

// FlagMessageRequest is the HTTP body for flagging a message
type FlagMessageRequest struct {
	Reason string `json:"reason" binding:"required"`
}

func createMessage(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	req := &CreateMessageRequest{}
	if err := c.BindJSON(req); err != nil {
		httpError(c, errors.Wrap(err, "could not bind JSON"))
		return
	}

	if !bson.IsObjectIdHex(req.Thread) {
		httpError(c, errors.New("invalid thread ID"))
		return
	}

	thread, err := store.GetThreads().WithID(bson.ObjectIdHex(req.Thread))
	lastMessageTime := thread.MessageTime
	if err != nil {
		httpError(c, err)
		return
	}

	if !thread.HasParticipant(*user) {
		httpError(c, errors.Wrap(err, "user not participant in thread"))
		return
	}

	if !req.Message.Type.Valid() {
		httpError(c, errors.Wrap(err, "invalid message type"))
		return
	}

	var message *store.Message
	var clean *func()

	switch req.Message.Type {
	case store.TypeText:
		message = store.GetMessages().NewMessage(user, thread.ID, req.Message.Type, req.Message.Body, nil)
	case store.TypeNotification:
		message = store.GetMessages().NewMessage(user, thread.ID, req.Message.Type, req.Message.Body, req.Message.Data)
	case store.TypeFile:

		id := bson.ObjectIdHex(req.Message.Body.(string))

		// it's possible that the student or tutor would add files from attachment to the library, retain this from cache.
		// see moveFileFromAttachmentHandler from me.go
		upload, err := services.Uploads.Get(id)
		if err != nil {
			httpError(c, errors.Wrap(err, "invalid upload key"))
			return
		}

		message = store.GetMessages().NewMessage(user, thread.ID, req.Message.Type, upload, nil)
	}

	if err := thread.AddMessage(message); err != nil {
		httpError(c, err)
		return
	}

	if len(thread.Participants) == 2 && len(thread.Deleted) > 0 {
		// FIXME:
		//if err := thread.UndeleteFor(core.User{ID: thread.Deleted[0]}); err != nil {
		//	httpError(c, err)
		//	return
		//}
	}

	if clean != nil {
		(*clean)()
	}

	go func() {
		message, err := store.GetMessages().WithID(message.ID)
		if err != nil {
			panic(err)
		}

		thread, err := store.GetThreads().WithID(thread.ID)
		if err != nil {
			panic(err)
		}

		ws.GetEngine().Notify(
			MsgEventMessageCreated,
			append(thread.ParticipantsIDS(), services.GetThreadObservers(thread.ID.Hex())...),
			ws.EventData{
				"message": message,
				"thread":  thread,
			},
		)

	}()

	d := delivery.New(config.GetConfig())
	for _, id := range thread.ParticipantsIDS() {
		if user.ID.Hex() != id {
			if other, ok := services.NewUsers().ByID(bson.ObjectIdHex(id)); ok {
				chatURL, err := core.AppURL(fmt.Sprintf("/main/inbox/%s", req.Thread))
				if err != nil {
					httpError(c, err)
				}

				if !utils.DateIsSame(lastMessageTime, message.Time) {
					go d.Send(other, m.TPL_MESSAGE_NOTIFICATION, &m.P{
						"FIRST_NAME": other.GetFirstName(),
						"OTHER_NAME": user.GetFirstName(),
						"CHAT_URL":   chatURL,
					})
				}
			}
		}

	}

	dto := message.GetDto(store.ThreadParticipant{
		ID: user.ID,
		Profile: store.ThreadParticipantProfile{
			FirstName: user.Profile.FirstName,
			LastName:  user.Profile.LastName,
			Avatar:    user.Profile.Avatar,
		},
	})

	c.JSON(http.StatusCreated, dto)
}

func routeFlagMessage(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	if !bson.IsObjectIdHex(c.Param("id")) {
		httpError(c, errors.New("invalid message ID"))
		return
	}

	req := &FlagMessageRequest{}
	if err := c.BindJSON(req); err != nil {
		httpError(c, errors.Wrap(err, "could not bind JSON"))
		return
	}

	message, err := store.GetMessages().WithID(bson.ObjectIdHex(c.Param("id")))
	if err != nil {
		httpError(c, err)
		return
	}

	if err := message.Flag(*user, req.Reason); err != nil {
		httpError(c, err)
		return
	}
}

func markMessageRead(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	var tomark []string
	if err := c.BindJSON(&tomark); err != nil {
		httpError(c, err)
		return
	}

	if err := store.GetMessages().MarkAsRead(tomark, user); err != nil {
		httpError(c, err)
		return
	}

	c.Status(http.StatusOK)
}

func countUsers(c *gin.Context) {

	user, exist := store.GetUser(c)
	if !exist {
		return
	}

	count, err := store.GetMessages().Count(user)
	if err != nil {
		httpError(c, err)
		return
	}

	var buf bytes.Buffer
	buf.WriteString(strconv.Itoa(count))
	c.Data(http.StatusOK, "text/plain", buf.Bytes())
}

func getOrCreateThreadAdmin(c *gin.Context) {
	req := struct {
		ReceiverID bson.ObjectId `json:"receiver" binding:"required"`
	}{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, err.Error())
		return
	}
	receiver, exist := services.NewUsers().ByID(req.ReceiverID)
	if !exist {
		c.JSON(http.StatusNotFound, "Receiver not found")
		return
	}
	sender, _ := store.GetUser(c)
	participants := []store.ThreadParticipant{*receiver.ToThreadParticipant(), *sender.ToThreadParticipant()}
	thread, err := getOrCreateThread(*sender.ToThreadParticipant(), participants, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, &thread)
}

func upload(c *gin.Context) {
	user, exist := store.GetUser(c)
	if !exist {
		return
	}
	context := c.Request.FormValue("context")
	download := c.Request.FormValue("download")
	accept := c.Request.FormValue("accept")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("File missing from request"))
		return
	}

	if accept != "" {
		match := false
		for _, v := range strings.Split(accept, ",") {
			if strings.HasSuffix(header.Filename, strings.Trim(v, " ")) {
				match = true
			}
		}
		if !match {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("File extension doesn't match what was sent in."))
			return
		}
	}

	// this is here to capture user, if there's valid user, retain it from s3. see Upload service
	upload, err := services.Uploads.Upload(user, context, header.Filename, &file, download == "true")
	if err != nil {
		err = errors.Wrap(err, "couldn't upload file")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	c.JSON(201, upload)
}

func Setup(g *gin.RouterGroup) {
	g.POST("/thread", auth.IsAdminMiddleware, getOrCreateThreadAdmin)
	g.GET("/threads", getThreads)
	g.POST("/threads", createThread)
	g.GET("/threads/:id", getThread)
	g.GET("/threads/:id/messages", routeGetThreadMessages)
	g.DELETE("/threads/:id", routeDeleteThread)
	g.POST("/threads/:id/archive", routeArchiveThread)
	g.POST("/threads/:id/flag", flagThread)

	g.POST("/messages", createMessage)
	g.POST("/messages/:id/flag", routeFlagMessage)

	g.POST("/upload", upload)

	g.GET("/count", countUsers)
	g.POST("/mark", markMessageRead)
}
