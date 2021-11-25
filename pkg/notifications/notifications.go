package notifications

import (
	"fmt"
	"strconv"
	"time"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services/delivery"
	m "gitlab.com/learnt/api/pkg/utils/messaging"

	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/ws"
	"gopkg.in/mgo.v2/bson"
)

type NotifyKind int

func (n NotifyKind) MarshalJSON() (data []byte, err error) {
	return []byte(strconv.Itoa(int(n))), nil
}

func (n *NotifyKind) UnmarshalJON(data []byte) error {
	parsed, err := strconv.ParseInt(string(data), 10, 0)
	if err != nil {
		return err
	}
	*n = NotifyKind(parsed)
	return nil
}

type NotificationMgo struct {
	ID      bson.ObjectId `json:"_id" bson:"_id"`
	Type    NotifyKind    `json:"type" bson:"type"`
	User    bson.ObjectId `json:"user" bson:"user"`
	Title   string        `json:"title" bson:"title"`
	Message string        `json:"message" bson:"message"`
	Action  *string       `json:"action,omitempty" bson:"action,omitempty"`
	Icon    *string       `json:"icon,omitempty" bson:"icon,omitempty"`
	Time    time.Time     `json:"time" bson:"time"`
	Data    interface{}   `json:"data,omitempty" bson:"data,omitempty"`
	Seen    bool          `json:"seen,omitempty" bson:"seen,omitempty"`
}

type Notification struct {
	ID      bson.ObjectId  `json:"_id" bson:"_id"`
	Type    NotifyKind     `json:"type" bson:"type"`
	User    *store.UserDto `json:"user" bson:"user" binding:"required"`
	Title   string         `json:"title" bson:"title" binding:"required"`
	Message string         `json:"message" bson:"message" binding:"required"`
	Action  *string        `json:"action,omitempty" bson:"action,omitempty"`
	Icon    *string        `json:"icon,omitempty" bson:"icon,omitempty"`
	Time    time.Time      `json:"time" bson:"time"`
	Data    interface{}    `json:"data,omitempty" bson:"data,omitempty"`
	Seen    bool           `json:"seen,omitempty" bson:"seen,omitempty"`
}

type PaginatedNotifications struct {
	Items  []Notification `json:"items" bson:"items"`
	Length int            `json:"length" bson:"length"`
}

type NotifyRequest struct {
	User    bson.ObjectId `json:"user" binding:"required"`
	Type    NotifyKind    `json:"type" binding:"required"`
	Title   string        `json:"title" binding:"required"`
	Message string        `json:"message" binding:"required"`
	Action  *string       `json:"action"`
	Icon    *string       `json:"icon"`
	Data    interface{}   `json:"data"`
}

type NotifyResponse struct {
	Succeed bool `json:"succeed"`
}

type Data map[string]interface{}

const (
	LessonBooked NotifyKind = iota + 1
	LessonAccepted
	LessonTutorCancelled
	LessonStudentCancelled
	LessonNotify24hBefore
	LessonNotify30mBefore
	LessonChangeRequest
	LessonStarted
	LessonSystemCancelled

	InstantLessonRequest
	InstantLessonAccept
	InstantLessonReject
	InstantLessonCancel

	LessonChangeRequestAccepted
	LessonChangeRequestDeclined

	LessonCompleteReview

	LessonNotifyBefore
	FavoriteTutorOnline
)

func Notify(request *NotifyRequest) (response chan *NotifyResponse) {
	response = make(chan *NotifyResponse)

	go func() {

		defer func() {

			err := recover()

			if err != nil {
				panic(err)
			}

			close(response)
		}()

		if request.Title == "" {
			logger.Get().Error("Notification title can't be empty")
			response <- &NotifyResponse{false}
			return
		}

		if request.Message == "" {
			logger.Get().Error("Notification message can't be empty")
			response <- &NotifyResponse{false}
			return
		}

		var user store.UserMgo
		if err := store.GetCollection("users").FindId(request.User).One(&user); err != nil {
			logger.Get().Error("User does not exist")
			response <- &NotifyResponse{false}
			return
		}

		logger.Get().Infof("Sending notification (%d) to %s: %s\n", request.Type, user.String(), request.Title)

		hub := ws.GetEngine().Hub

		n := Notification{
			ID:      bson.NewObjectId(),
			Type:    request.Type,
			User:    user.Dto(),
			Title:   request.Title,
			Message: request.Message,
			Action:  request.Action,
			Data:    request.Data,
			Icon:    request.Icon,
			Time:    time.Now(),
		}

		notification := &NotificationMgo{
			ID:      n.ID,
			Type:    n.Type,
			User:    user.ID,
			Title:   n.Title,
			Message: n.Message,
			Action:  n.Action,
			Data:    n.Data,
			Icon:    n.Icon,
			Time:    n.Time,
		}

		if err := store.GetCollection("notifications").Insert(notification); err != nil {
			logger.Get().Error("Fail to store notification", err)
			response <- &NotifyResponse{false}
			return
		}
		c := hub.User(request.User)
		if c != nil {
			if err := c.Send(ws.Event{
				Type: "notification",
				Data: ws.EventData{"notification": n},
			}); err != nil {
				response <- &NotifyResponse{false}
			} else {
				response <- &NotifyResponse{true}
			}
		} else {
			response <- &NotifyResponse{false}
		}
	}()

	return response
}

// ForUser gets the notifications for a user
func ForUser(user *store.UserMgo, limit, offset int) (*PaginatedNotifications, error) {

	query := store.GetCollection("notifications").
		Find(bson.M{"user": user.ID}).
		Sort("-time")

	length, err := query.Count()
	if err != nil {
		return nil, errors.Wrap(err, "could not get notifications count")
	}

	var items []Notification
	if err := query.Skip(offset).Limit(limit).All(&items); err != nil {
		return nil, errors.Wrap(err, "could not get notifications count")
	}

	pn := &PaginatedNotifications{
		Items:  items,
		Length: length,
	}
	return pn, nil
}

func NotifyFollowers(tutor *store.UserMgo) {
	for _, student := range tutor.Favorite.Students {
		var user *store.UserMgo
		if err := store.GetCollection("users").FindId(student.Student).One(&user); err != nil {
			continue
		}

		// Notify on the site while loggedin
		Notify(&NotifyRequest{
			User:    student.Student,
			Type:    FavoriteTutorOnline,
			Title:   "Favorite Tutor",
			Message: fmt.Sprintf("%s %s is online now", tutor.Profile.FirstName, tutor.Profile.LastName),
			Data:    map[string]interface{}{"tutor": tutor.ID.Hex()},
		})

		// Notify via email and sms
		tutorProfileLink, err := core.AppURL("/main/tutor/%s", tutor.ID.Hex())
		if err != nil {
			return
		}

		d := delivery.New(config.GetConfig())
		// let this finish sending
		if err := d.Send(user, m.TPL_TUTOR_ONLINE_NOW, &m.P{
			"FIRST_NAME":    user.Profile.FirstName,
			"TUTOR_NAME":    tutor.Profile.FirstName,
			"TUTOR_PROFILE": tutorProfileLink,
		}); err != nil {
			return
		}

	}
}
