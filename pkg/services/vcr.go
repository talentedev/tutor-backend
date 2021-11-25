package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services/delivery"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/utils/messaging/mail"

	jose "github.com/dvsekhvalnov/jose2go"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/notifications"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/ws"
	"gopkg.in/mgo.v2/bson"
)

const (
	ROOM_KEEP_ALIVE = time.Minute
	FiveMinutes     = 5 * time.Minute
	TwoMinutes      = 2 * time.Minute
)

// Room is a virtual classroom

type CanvasObject map[string]interface{}

func (co CanvasObject) ID() string {

	id, exist := co["id"]
	if !exist {
		return "?"
	}

	return id.(string)
}

type WhiteboardCanvas struct {
	ID      string         `json:"id" bson:"id"`
	Name    string         `json:"name" bson:"name"`
	Objects []CanvasObject `json:"objects" bson:"objects"`
	hindex  int
}

func (wc *WhiteboardCanvas) Add(o CanvasObject) {
	wc.Objects = append(wc.Objects, o)
	wc.hindex++
}

func (wc *WhiteboardCanvas) Undo() {

}

func (wc *WhiteboardCanvas) Redo() {

}

type Whiteboard struct {
	Sessions []*WhiteboardCanvas `json:"sessions" bson:"sessions"`
	Active   string              `json:"active" bson:"active"`
}

func (wb *Whiteboard) Session(session string) *WhiteboardCanvas {

	for _, s := range wb.Sessions {
		if s.ID == session {
			return s
		}
	}

	panic("Session not found")
}

func (wb *Whiteboard) Create(session string, o CanvasObject) {
	wb.Session(session).Add(o)
}

func (wb *Whiteboard) Get(session string, object string) (obj CanvasObject, err error) {

	if s := wb.Session(session); s != nil {
		for _, o := range s.Objects {
			if o.ID() == object {
				return o, nil
			}
		}
	}

	return nil, errors.New("Object not found")
}

func (wb *Whiteboard) Update(session string, object string, src CanvasObject) (err error) {

	if s := wb.Session(session); s != nil {
		for i, o := range s.Objects {
			if o.ID() == object {
				s.Objects[i] = src
				return
			}
		}
	}

	return errors.New("Object not found")
}

func (wb *Whiteboard) Remove(session string, object string) (err error) {

	if s := wb.Session(session); s != nil {
		for i, o := range s.Objects {
			if o.ID() == object {
				s.Objects = append(s.Objects[:i], s.Objects[i+1:]...)
				return
			}
		}
	}

	return errors.New("Object not found")
}

type Code struct {
	Value    string `json:"value"`
	Settings struct {
		Theme    string `json:"theme"`
		Language string `json:"language"`
	} `json:"settings"`
}

type Session struct {
	Room *store.RoomEntity `json:"dto"`

	Tutor     *ws.Connection            `json:"tutor"`
	Students  map[string]*ws.Connection `json:"students"`
	Observers map[string]*ws.Connection `json:"observers"`

	Module string `json:"module"`
	*Whiteboard
	*Code `json:"code"`
	Text  string            `json:"text"`
	Peers map[string]string `json:"peers"`

	mux sync.Mutex
}

func (r *Session) UserConnected(user store.UserMgo) bool {

	for _, c := range r.Connections() {
		if c.GetUser().ID.Hex() == user.ID.Hex() {
			return true
		}
	}

	return false
}

func (r *Session) Connections() (cns []*ws.Connection) {

	if r.Tutor != nil {
		cns = append(cns, r.Tutor)
	}

	for _, student := range r.Students {
		cns = append(cns, student)
	}

	for _, observer := range r.Observers {
		cns = append(cns, observer)
	}

	return
}

// Remove removes a websocket connection from a room
func (r *Session) Remove(c *ws.Connection) (deleted bool) {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.Tutor == c {
		r.Tutor = nil
		deleted = true
	}

	for key, student := range r.Students {
		if student.GetUser().ID.Hex() == c.GetUser().ID.Hex() {
			delete(r.Students, key)
			deleted = true
		}
	}

	for key, observer := range r.Observers {
		if observer.GetUser().ID.Hex() == c.GetUser().ID.Hex() {
			delete(r.Observers, key)
			deleted = true
		}
	}

	return
}

// Dispatch dispatches an event in a room to all connected users
func (r *Session) Dispatch(eventType string, data ws.EventData, except ...*ws.Connection) {

	event := ws.Event{Type: eventType, Data: data}

	for _, c := range r.Connections() {

		if len(except) > 0 && except[0] == c {
			continue
		}

		c.Send(event)
	}
}

func (r *Session) isEmpty() bool {
	return len(r.Connections()) == 0
}

func (r *Session) Ready() bool {
	return len(r.Connections()) == 2
}

// VCR is the engine that holds all virtual classroms
type VCR struct {
	engine *ws.Engine
	rooms  map[string]*Session
	// Pending closes after ROOM_KEEP_ALIVE on user leave
	closes map[string]bool
	mux    MutexGroup
}

var vcr *VCR

func (vcr *VCR) GetRooms() map[string]*Session {
	return vcr.rooms
}

// NewActivity adds an activity to a room and saves it in the db
func (vcr *VCR) NewActivity(room *Session, user *store.UserMgo, action store.ActivityAction) (err error) {
	var userID *bson.ObjectId

	if user != nil {
		userID = &user.ID
	}

	activity := store.RoomActivity{
		User:   userID,
		Action: action,
		Time:   time.Now(),
	}

	room.Room.Activity = append(room.Room.Activity, activity)

	err = store.GetCollection("rooms").UpdateId(room.Room.ID, bson.M{"$push": bson.M{"activity": activity}})

	return
}

// InitVCR sets up the virtual classroom engine
func InitVCR(ctx context.Context) {
	logger.Get().Info("Virtual Class Room: Init")

	vcr = &VCR{
		engine: ws.GetEngine(),
		rooms:  make(map[string]*Session),
		closes: make(map[string]bool),
		mux:    MutexGroup{},
	}

	vcr.SyncWithStorage()

	//@listeners
	vcr.Listen("vcr.join", vcr.Join)
	vcr.Listen("vcr.module", vcr.onModuleChange)
	vcr.Listen("vcr.text.sync", vcr.onTextSync)
	vcr.Listen("vcr.text.release", vcr.forward)
	vcr.Listen("vcr.code.sync", vcr.onCodeSync)
	vcr.Listen("vcr.code.release", vcr.forward)
	vcr.Listen("vcr.wb.remove", vcr.onWbSessionRemove)
	vcr.Listen("vcr.wb.create", vcr.onWbSessionCreate)
	vcr.Listen("vcr.wb.active", vcr.onWbSessionActive)
	vcr.Listen("vcr.wb.undo", vcr.onWbSessionUndo)
	vcr.Listen("vcr.wb.redo", vcr.onWbSessionRedo)
	vcr.Listen("vcr.wb.name", vcr.onWbSessionName)
	vcr.Listen("vcr.end", vcr.onEndSession)
	vcr.Listen("vcr.end.incomplete", vcr.onEndIncompleteSession)
	vcr.Listen("vcr.extend.time", vcr.onExtendTime)
	vcr.Listen("vcr.wait", vcr.onWait)
	vcr.Listen("vcr.wait.cancel", vcr.onWaitCancel)
	vcr.Listen("vcr.extend.time.accept", vcr.onExtendTimeAccept)
	vcr.Listen("vcr.extend.time.reject", vcr.onExtendTimeReject)
	vcr.Listen("vcr.peer.connected", vcr.SetPeer)

	vcr.engine.UserLeave(vcr.onUserLeave)
}

func (vcr *VCR) Listen(event string, f func(event ws.Event, room *Session, engine *ws.Engine)) {
	vcr.engine.Listen(event, func(event ws.Event, engine *ws.Engine) {
		defer func() {
			if err := recover(); err != nil {
				logger.Get().Error("recovered panic on vcr socket handler:", err, "event:", event)
			}
		}()

		room, err := vcr.getRoomFromEvent(event)

		if err != nil {
			event.Error(err.Error(), 100)
			return
		}

		f(event, room, engine)
	})
}

func (vcr *VCR) onUserLeave(c *ws.Connection) {

	if room := vcr.GetConnectionRoom(c); room != nil {
		if room.Remove(c) {

			user := c.GetUser()

			// observer leaves
			if user.IsAdmin() {
				return
			}

			if err := vcr.NewActivity(room, user, store.ROOM_ACTIVITY_LEAVE); err != nil {
				panic(err)
			}

			// mark room for closing if user
			// does not enter in ROOM_KEEP_ALIVE
			vcr.closes[room.Room.ID.Hex()] = true

			go func() {

				time.Sleep(ROOM_KEEP_ALIVE)

				if _, closePending := vcr.closes[room.Room.ID.Hex()]; closePending {
					if room.Room.CompletedAt.IsZero() {
						vcr.endSession(room)
					}
				}

			}()

			room.Dispatch("vcr.leave", ws.EventData{
				"user":       user.ID.Hex(),
				"keep_alive": ROOM_KEEP_ALIVE.Seconds(),
			})

			logger.Get().Info("User", user, "left room", room.Room.String())
		} else {
			logger.Get().Error("VCR: Failed to remove the user from the room")
		}
	} else {
		vcr.DumpLog()
		logger.Get().Error("VCR: No room found for the user on leave. Active rooms:", vcr.RoomCount())
	}
}

// DumpLog logs details about the VCR
func (vcr *VCR) DumpLog() {
	logger.Get().Info("> ------ ROOMS ------")
	for _, room := range vcr.rooms {
		logger.Get().Info("Room:", room.Room.String())

		for _, student := range room.Students {
			logger.Get().Info("Students:", student.GetUser().ID.Hex())
		}

		if room.Tutor != nil {
			logger.Get().Info("Tutor:", room.Tutor.GetUser().ID.Hex())
		}
	}
	logger.Get().Info("< ------ ROOMS ------")
}

func (vcr *VCR) isSameConnectionUser(a, b *ws.Connection) bool {
	if a == nil || b == nil {
		return false
	}

	if a.GetUser() == nil || b.GetUser() == nil {
		return false
	}

	return a.GetUser().ID.Hex() == b.GetUser().ID.Hex()
}

// GetConnectionRoom gets the room from a connection
func (vcr *VCR) GetConnectionRoom(c *ws.Connection) *Session {

	for _, room := range vcr.rooms {
		if vcr.isSameConnectionUser(room.Tutor, c) {
			return room
		}

		for _, student := range room.Students {
			if vcr.isSameConnectionUser(student, c) {
				return room
			}
		}

		for _, observer := range room.Observers {
			if vcr.isSameConnectionUser(observer, c) {
				return room
			}
		}
	}

	return nil
}

// RoomCount lists the number of current virtual classrooms
func (vcr *VCR) RoomCount() int {
	return len(vcr.rooms)
}

// SyncWithStorage syncs memory active class rooms with mongo rooms collection
func (vcr *VCR) SyncWithStorage() {
	if vcr == nil {
		return
	}

	vcr.mux.Get("sync").Lock()
	defer vcr.mux.Get("sync").Unlock()

	rooms := make([]*store.RoomEntity, 0)

	query := store.GetCollection("rooms").Find(
		bson.M{"completed_at": bson.M{"$exists": false}},
	)

	query.All(&rooms)

	vcr.rooms = make(map[string]*Session, 0)

	updated := 0

	for _, room := range rooms {

		session := &Session{
			Room:      room,
			Students:  make(map[string]*ws.Connection),
			Observers: make(map[string]*ws.Connection),
			Whiteboard: &Whiteboard{
				Sessions: make([]*WhiteboardCanvas, 0),
			},
			Code: &Code{
				Value: "",
			},
			Peers: make(map[string]string, 0),
		}

		vcr.rooms[room.ID.Hex()] = session

		updated++
	}

	logger.Get().Info("Virtual Class Room: Rooms synced with storage:", updated)
}

// CreateConversationThread creates a new conversation to use the a room for a lesson
func (vcr *VCR) CreateConversationThread(lesson *store.LessonMgo) (id bson.ObjectId, err error) {

	participants := make([]bson.ObjectId, 0)

	students, err := lesson.StudentsDto()
	if err != nil {
		return id, err
	}

	for _, user := range students {
		participants = append(participants, user.ID)
	}

	tutorDto := lesson.TutorDto()
	if err != nil {
		return id, err
	}

	participants = append(participants, tutorDto.ID)

	var body string

	if lesson.IsInstantSession() {
		body = "Room for instant session is created"
	} else {
		body = "Room created"
	}

	thread, err := store.GetThreads().Create(&store.Thread{
		ID:           bson.NewObjectId(),
		Name:         lesson.FetchSubjectName(),
		Participants: participants,
		Creator:      tutorDto.ID,
		Time:         time.Now(),
	})

	if err != nil {
		return id, err
	}

	thread.AddMessage(&store.Message{
		ID:     bson.NewObjectId(),
		Sender: tutorDto.ID,
		Body:   body,
		Type:   "text",
	})

	return thread.ID, nil
}

// GetRoom creates a new virtual classrom in the VCR engine
func (vcr *VCR) GetRoomForLesson(lesson *store.LessonMgo) (room *store.RoomEntity, err error) {

	if lesson.Room != nil {
		err = store.GetCollection("rooms").FindId(lesson.Room).One(&room)
		return
	}

	if !lesson.IsConfirmed() {
		return nil, errors.New("Lesson must be in confirmed state")
	}

	if lesson.StartsAt.After(time.Now()) {
		return nil, errors.New("Virtual class room for this lesson can be created only after lesson booked time")
	}

	thread, err := vcr.CreateConversationThread(lesson)
	if err != nil {
		return nil, errors.Wrap(err, "error creating conversation thread")
	}

	room = &store.RoomEntity{
		ID:       bson.NewObjectId(),
		LessonID: lesson.ID,
		Thread:   thread,
		Activity: make([]store.RoomActivity, 0),
	}

	if err = room.Save(); err != nil {
		return nil, errors.Wrap(err, "Failed to save the the room in storage system")
	}

	if err := lesson.SetRoom(room.ID); err != nil {
		return nil, errors.Wrap(err, "Failed to set lesson room id")
	}

	vcr.SyncWithStorage()

	return
}

func (vcr *VCR) getRoomFromEvent(event ws.Event) (room *Session, err error) {

	roomID := event.GetString("room")

	if roomID == "" {
		return nil, errors.New("Room id is missing from the request")
	}

	if event.Source == nil {
		return nil, errors.New("Source connection is missing from the request")
	}

	room, exist := vcr.rooms[roomID]

	if !exist {
		return nil, errors.New("Room with this id does not exist")
	}

	return room, nil
}

func (vcr *VCR) onModuleChange(event ws.Event, room *Session, engine *ws.Engine) {
	room.Module = event.GetString("name")
	room.Dispatch(event.Type, ws.EventData{
		"user": event.Source.GetUser().ID.Hex(),
		"name": event.GetString("name"),
	}, event.Source)
}

func (vcr *VCR) onExtendTime(event ws.Event, room *Session, engine *ws.Engine) {
	logger.Get().Info("On extend time: ", event.GetString("time"))

	room.Dispatch(event.Type, ws.EventData{
		"user": event.Source.GetUser().ID.Hex(),
		"time": event.GetString("time"),
	}, event.Source)
}

func (vcr *VCR) onWait(event ws.Event, room *Session, engine *ws.Engine) {
	logger.Get().Info("On wait: ", FiveMinutes)

	var other int
	for i, participant := range room.Room.GetLesson().GetParticipants() {
		if participant.ID.Hex() != event.Source.GetUser().ID.Hex() {
			other = i
		}
	}

	conf := config.GetConfig()
	d := delivery.New(conf)

	roomURL, err := core.AppURL("/room/%s", room.Room.ID.Hex())
	if err != nil {
		logger.Get().Info(err.Error())
	}

	go d.Send(event.Source.GetUser(), m.TPL_FIVE_MINUTES_LATE, &m.P{
		"FIRST_NAME":    event.Source.GetUser().GetFirstName(),
		"OTHER_NAME":    room.Room.GetLesson().GetParticipants()[other].Name(),
		"CLASSROOM_URL": roomURL,
	})
}

func (vcr *VCR) onWaitCancel(event ws.Event, room *Session, engine *ws.Engine) {
	logger.Get().Info("On wait cancelled.")

	tutor := room.Room.Lesson.TutorDto()
	if err := GetLessons().CompleteUnAuthorized(room.Room.Lesson); err != nil {
		panic(err)
	}

	room.Room.CompletedAt = time.Now()

	vcr.saveRoomData(room)

	for _, c := range room.Connections() {
		c.Close()
	}

	var isTutor bool
	var other int

	for i, participant := range room.Room.GetLesson().GetParticipants() {
		if participant.ID.Hex() != event.Source.GetUser().ID.Hex() {
			other = i
		}
	}

	isTutor = event.Source.GetUser().ID.Hex() == tutor.ID.Hex()

	conf := config.GetConfig()
	d := delivery.New(conf)

	if isTutor {
		d.Send(event.Source.GetUser(), m.TPL_LESSON_NO_SHOW_STUDENT, &m.P{
			"STUDENT_NAME": room.Room.GetLesson().GetParticipants()[other].Name(),
		})

		mail.GetSender(conf).SendTo(m.HIRING_EMAIL, m.TPL_LESSON_NO_SHOW_ADMIN, &m.P{
			"TUTOR_NAME":   event.Source.GetUser().Name(),
			"STUDENT_NAME": room.Room.GetLesson().GetParticipants()[other].Name(),
		})
	} else {
		d.Send(event.Source.GetUser(), m.TPL_LESSON_NO_SHOW_TUTOR, &m.P{
			"TUTOR_NAME":   room.Room.GetLesson().GetParticipants()[other].Name(),
			"STUDENT_NAME": event.Source.GetUser().Name(),
		})

		mail.GetSender(conf).SendTo(m.HIRING_EMAIL, m.TPL_LESSON_NO_SHOW_ADMIN, &m.P{
			"TUTOR_NAME":           room.Room.GetLesson().GetParticipants()[other].Name(),
			"STUDENT_NAME":         event.Source.GetUser().Name(),
			"MERGE_LESSON_DETAILS": room.Room.GetLesson().FetchSubjectName(),
		})
	}

	logger.Get().Infof("Virtual class room %s completed", room.Room.ID.Hex())
}

func (vcr *VCR) onWaitReject(event ws.Event, room *Session, engine *ws.Engine) {
	logger.Get().Info("On wait cancelled.")

}

func (vcr *VCR) onExtendTimeAccept(event ws.Event, room *Session, engine *ws.Engine) {

	tLayout := "2006-01-02T15:04:05Z"
	timeEndStr := event.GetString("timeEnd")
	timeEnd, err := time.Parse(tLayout, timeEndStr)

	if err != nil {
		logger.Get().Errorf("onExtendTimeAccept Date Parse error: %v", err)
		event.Error("Date parse error", 0)
		return
	}

	if err := room.Room.Lesson.SetEndsAt(timeEnd); err != nil {
		logger.Get().Errorf("onExtendTimeAccept SetEndsAt error: %v", err)
		event.Error("Couldn't set ends_at date", 0)
		return
	}

	thread, err := store.GetThreads().WithID(room.Room.Thread)
	if err != nil {
		logger.Get().Errorf("onExtendTimeAccept GetThreads error: %v", err)
		event.Error("Couldn't fetch message thread", 0)
		return
	}

	messages := store.GetMessages().ForThread(thread, 0, 15)

	for _, message := range messages {
		if file, ok := message.Body.(*store.Upload); ok {
			upload, err := Uploads.Get(file.ID)
			if err != nil {
				logger.Get().Errorf("onExtendTimeAccept GetThreads error: %v", err)
				event.Error("Couldn't fetch file from message thread", 0)
				return
			}

			if upload != nil {
				expire := upload.Expire.Add(time.Hour*time.Duration(timeEnd.Hour()) +
					time.Minute*time.Duration(timeEnd.Minute()) +
					time.Second*time.Duration(timeEnd.Second()))
				upload.Expire = &expire
			}

		}
	}

	room.Dispatch(event.Type, ws.EventData{
		"user":    event.Source.GetUser().ID.Hex(),
		"timeEnd": timeEnd,
	})
}

func (vcr *VCR) onExtendTimeReject(event ws.Event, room *Session, engine *ws.Engine) {

	room.Dispatch(event.Type, ws.EventData{
		"user": event.Source.GetUser().ID.Hex(),
	}, event.Source)
}

func (vcr *VCR) forward(event ws.Event, room *Session, engine *ws.Engine) {
	room.Dispatch(event.Type, event.Data, event.Source)
}

func (vcr *VCR) onCodeSync(event ws.Event, room *Session, engine *ws.Engine) {

	if room.Code == nil {
		room.Code = &Code{}
	}

	code := event.GetString("code")
	theme := event.GetString("theme")
	language := event.GetString("language")

	evData := ws.EventData{}

	if code != "" {
		room.Code.Value = code
		evData["code"] = code
	}

	if theme != "" {
		room.Code.Settings.Theme = theme
		evData["theme"] = theme
	}

	if language != "" {
		room.Code.Settings.Language = language
		evData["language"] = language
	}

	go vcr.saveRoomData(room)

	room.Dispatch(event.Type, evData, event.Source)
}

func (vcr *VCR) onTextSync(event ws.Event, room *Session, engine *ws.Engine) {

	room.Text = event.GetString("text")

	go vcr.saveRoomData(room)

	room.Dispatch(event.Type, event.Data, event.Source)
}

func (vcr *VCR) onWbSessionCreate(event ws.Event, room *Session, engine *ws.Engine) {

	event.MustHave("session", "name")

	session := &WhiteboardCanvas{
		ID:      event.GetString("session"),
		Objects: make([]CanvasObject, 0),
		Name:    event.GetString("name"),
	}

	room.Whiteboard.Sessions = append(room.Whiteboard.Sessions, session)

	room.Dispatch(event.Type, event.Data, event.Source)

	go vcr.saveRoomData(room)
}

func (vcr *VCR) GetRoom(id string) (room *Session) {
	for _, r := range vcr.rooms {
		if r.Room.ID.Hex() == id {
			return r
		}
	}
	return
}

func (vcr *VCR) GetEngine() *ws.Engine {
	return vcr.engine
}

func (vcr *VCR) onWbSessionActive(event ws.Event, room *Session, engine *ws.Engine) {
	event.MustHave("session")
	room.Whiteboard.Active = event.GetString("session")
	room.Dispatch(event.Type, event.Data, event.Source)
	go vcr.saveRoomData(room)
}

func (vcr *VCR) onWbSessionUndo(event ws.Event, room *Session, engine *ws.Engine) {
	event.MustHave("session")
	room.Whiteboard.Session((event.GetString("session"))).Undo()
	room.Dispatch(event.Type, event.Data, event.Source)
}

func (vcr *VCR) onWbSessionRedo(event ws.Event, room *Session, engine *ws.Engine) {
	event.MustHave("session")
	room.Whiteboard.Session((event.GetString("session"))).Redo()
	room.Dispatch(event.Type, event.Data, event.Source)
}

func (vcr *VCR) onWbSessionName(event ws.Event, room *Session, engine *ws.Engine) {

	event.MustHave("session", "name")

	id := event.GetString("session")

	for i, session := range room.Whiteboard.Sessions {
		if session.ID == id {
			room.Whiteboard.Sessions[i].Name = event.GetString("name")
		}
	}

	room.Dispatch(event.Type, event.Data, event.Source)
	go vcr.saveRoomData(room)
}

func (vcr *VCR) onWbSessionRemove(event ws.Event, room *Session, engine *ws.Engine) {

	event.MustHave("session")

	id := event.GetString("session")

	for i, session := range room.Whiteboard.Sessions {
		if session.ID == id {
			room.Whiteboard.Sessions = append(room.Whiteboard.Sessions[:i], room.Whiteboard.Sessions[i+1:]...)
		}
	}

	go vcr.saveRoomData(room)

	room.Dispatch(event.Type, event.Data, event.Source)
}

func (vcr *VCR) saveRoomData(room *Session) {

	vcr.mux.Get("save").Lock()
	defer vcr.mux.Get("save").Unlock()

	store.GetCollection("rooms").UpdateId(
		room.Room.ID,
		bson.M{
			"$set": bson.M{
				"whiteboard":   room.Whiteboard.Sessions,
				"code":         room.Code,
				"text":         room.Text,
				"completed_at": room.Room.CompletedAt,
			},
		},
	)

}

func (vcr *VCR) endSession(room *Session, connections ...*ws.Connection) {

	logger.Get().Debugf("Ending session %v", room.Room.ID)

	// if admin ends session, only close connection
	if len(connections) > 0 {
		user := connections[0].GetUser()
		if user.IsAdmin() {
			for _, c := range room.Connections() {
				if c.GetUser() == user {
					c.Close()
				}
			}
			return
		}
	}

	tutor := room.Room.Lesson.TutorDto()

	room.Dispatch("vcr.end", nil)

	if err := GetLessons().Complete(room.Room.Lesson); err != nil {
		panic(err)
	}

	room.Room.CompletedAt = time.Now()

	vcr.saveRoomData(room)

	for _, c := range room.Connections() {
		c.Close()
	}

	for _, participant := range room.Room.GetLesson().GetParticipants() {

		if participant.ID.Hex() == tutor.ID.Hex() {
			continue
		}

		if _, yes := tutor.GetReviewFrom(participant.ToPublicDto()); yes {
			continue
		}

		notifications.Notify(&notifications.NotifyRequest{
			User:    participant.ID,
			Type:    notifications.LessonCompleteReview,
			Title:   "Leave a review",
			Message: fmt.Sprintf("What do you think about %s %s?", tutor.Profile.FirstName, tutor.Profile.LastName),
			Data:    map[string]interface{}{"user": tutor},
		})
	}
	// this ensures it's on not on the queue, otherwise after ROOM_KEEP_ALIVE, onUserLeave will kick-in and will flow back here again and re-charge the tutor.
	delete(vcr.closes, room.Room.ID.Hex())

	logger.Get().Infof("Virtual class room %s completed", room.Room.ID.Hex())
}

func (vcr *VCR) endIncompleteSession(room *Session) {
	logger.Get().Debugf("Ending incomplete session %v", room.Room.ID)

	room.Dispatch("vcr.end", nil)

	if err := GetLessons().CompleteUnAuthorized(room.Room.Lesson); err != nil {
		panic(err)
	}

	room.Room.CompletedAt = time.Now()

	vcr.saveRoomData(room)

	for _, c := range room.Connections() {
		c.Close()
	}
	// this ensures it's on not on the queue, otherwise after ROOM_KEEP_ALIVE, onUserLeave will kick-in and will flow back here again and re-charge the tutor.
	delete(vcr.closes, room.Room.ID.Hex())

	logger.Get().Infof("Virtual class room %s completed", room.Room.ID.Hex())
}

func (vcr *VCR) onEndSession(event ws.Event, room *Session, engine *ws.Engine) {
	vcr.endSession(room, event.Source)
}

func (vcr *VCR) onEndIncompleteSession(event ws.Event, room *Session, engine *ws.Engine) {
	vcr.endIncompleteSession(room)
}

// Join connects a user to a room
func (vcr *VCR) Join(event ws.Event, room *Session, engine *ws.Engine) {
	user := event.Source.GetUser()
	isObserver := user.IsAdmin()
	if room.UserConnected(*user) {
		event.Error("Only one virtual room instance is allowed", 0)
		return
	}

	if !room.Room.CompletedAt.IsZero() {
		event.Error("Room is not live anymore", 0)
		return
	}

	if !isObserver && !room.Room.GetLesson().HasUser(user) {
		event.Error("Not a valid participant for this lesson room", 0)
		return
	}

	//token, err := vcr.GenerateRoomToken(event.Source.GetUser(), room)

	//if err != nil {
	//	event.Error("Failed to generate token for this room", 0)
	//	return
	//}

	if user.ID.Hex() == room.Room.GetLesson().Tutor.Hex() {
		room.Tutor = event.Source
	}

	if !isObserver && user.ID.Hex() != room.Room.GetLesson().Tutor.Hex() {
		room.mux.Lock()
		room.Students[event.Source.GetUser().ID.Hex()] = event.Source
		room.mux.Unlock()
	}

	if isObserver {
		room.mux.Lock()
		room.Observers[event.Source.GetUser().ID.Hex()] = event.Source
		room.mux.Unlock()
	}

	if !isObserver {
		vcr.NewActivity(room, event.Source.GetUser(), "enter")
	}

	if _, closePending := vcr.closes[room.Room.ID.Hex()]; closePending {
		delete(vcr.closes, room.Room.ID.Hex())
	}

	logger.Get().Info("Virtual Class Room: User join", event.Source.GetUser().String(), "at ", room.Room.GetLesson())

	students := make([]*store.UserDto, 0)

	for _, user := range room.Students {
		students = append(students, user.GetUser().Dto())
	}

	var tutor *store.UserDto

	if room.Tutor != nil {
		tutor = room.Tutor.GetUser().Dto()
	}

	if room.Module == "" {
		room.Module = "whiteboard"

		if len(room.Whiteboard.Sessions) == 0 {

			id := bson.NewObjectId().Hex()
			session := &WhiteboardCanvas{
				ID:      id,
				Objects: make([]CanvasObject, 0),
				Name:    "Page 1",
			}
			room.Whiteboard.Active = id
			room.Whiteboard.Sessions = []*WhiteboardCanvas{session}
		}

		room.Dispatch("vcr.wb.sync", nil, event.Source)
	}

	if !isObserver {
		room.Dispatch("vcr.enter", ws.EventData{
			"user": event.Source.GetUser().Dto(true),
		})
	}

	evData := ws.EventData{
		"room":     room.Room,
		"tutor":    tutor,
		"students": students,
		"module":   room.Module,
	}

	if !room.Ready() {
		evData["waiting"] = FiveMinutes.Seconds()
	}

	event.Respond("vcr.join.ok", evData)
}

// GenerateRoomToken TODO: remove no longer used
func (vcr *VCR) GenerateRoomToken(user *store.UserMgo, room *Session) (token string, err error) {

	cfg := config.GetConfig()

	twillioAccount := strings.Trim(cfg.GetString("service.twilio.account"), " ")
	twilioToken := strings.Trim(cfg.GetString("service.twilio.token"), " ")
	twilioSecret := strings.Trim(cfg.GetString("service.twilio.secret"), " ")

	if twillioAccount == "" || twilioToken == "" {
		return "", errors.New("Twilio credentials missing")
	}

	payloadData := map[string]interface{}{
		"jti":   fmt.Sprintf("%s-%d", twilioToken, time.Now().Unix()),
		"iss":   twilioToken,
		"sub":   twillioAccount,
		"jwtid": "jti",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(time.Hour).Unix(),
		"grants": map[string]interface{}{
			"identity": user.ID.Hex(),
			"video": map[string]interface{}{
				"room": room.Room.ID.Hex(),
			},
		},
	}

	payloadString, err := json.Marshal(payloadData)
	if err != nil {
		return "", err
	}

	accessToken, err := jose.SignBytes(
		payloadString,
		jose.HS256,
		[]byte(twilioSecret),
		jose.Header("cty", "twilio-fpa;v=1"),
		jose.Header("typ", "JWT"),
	)

	return accessToken, err
}

// Stats gets stats about the virtual classrooms container
func (vcr *VCR) Stats() interface{} {
	stats := make(map[string]interface{})
	stats["rooms"] = vcr.rooms
	return stats
}

func (vcr *VCR) SetPeer(event ws.Event, room *Session, engine *ws.Engine) {
	peerId := event.Data["id"].(string)
	user := event.Source.GetUser()
	room.Peers[user.ID.Hex()] = peerId
	room.Dispatch("vcr.peer.joined", ws.EventData{
		"peers": room.Peers,
	})
}

// VCRInstance gets the virtual classroom engine
func VCRInstance() *VCR {
	return vcr
}

type MutexGroup struct {
	group map[string]*sync.Mutex
}

func (mg *MutexGroup) Get(name string) (mux *sync.Mutex) {

	if mg.group == nil {
		mg.group = make(map[string]*sync.Mutex)
	}

	if mux, exist := mg.group[name]; exist {
		return mux
	}

	mux = new(sync.Mutex)

	mg.group[name] = mux

	return
}
