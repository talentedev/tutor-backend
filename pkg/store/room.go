package store

import (
	"fmt"
	"sync"
	"time"

	"gitlab.com/learnt/api/pkg/logger"
	"gopkg.in/mgo.v2/bson"
)

type ActivityAction string

type RoomActivity struct {
	User   *bson.ObjectId `json:"user" bson:"user"`
	Action ActivityAction `json:"action" bson:"action"`
	Time   time.Time      `json:"time" bson:"time"`
}

const (
	ROOM_ACTIVITY_EDIT  ActivityAction = "edit"
	ROOM_ACTIVITY_ENTER ActivityAction = "enter"
	ROOM_ACTIVITY_LEAVE ActivityAction = "leave"
)

type RoomEntity struct {
	ID bson.ObjectId `json:"_id" bson:"_id"`

	LessonID bson.ObjectId `json:"-" bson:"lesson"`
	Lesson   *LessonMgo    `json:"lesson" bson:"-"`

	Activity []RoomActivity `json:"activity" bson:"activity"`
	Thread   bson.ObjectId  `json:"thread" bson:"thread"`

	Code       string   `json:"code,omitempty" bson:"code,omitempty"`
	Text       string   `json:"text,omitempty" bson:"text,omitempty"`
	Whiteboard []string `json:"whiteboard,omitempty" bson:"whiteboard,omitempty"`

	CreatedAt   time.Time `json:"created_at,omitempty" bson:"created_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`

	mux sync.Mutex
}

func (r *RoomEntity) GetLesson() *LessonMgo {

	if r.Lesson != nil {
		return r.Lesson
	}

	if err := GetCollection("lessons").FindId(r.LessonID).One(&r.Lesson); err != nil {
		logger.Get().Errorf("couldn't get db lessons in room Dto: %v", err)
	}

	return r.Lesson
}

func (r *RoomEntity) String() string {
	return fmt.Sprintf("[Room %s]", r.ID.Hex())
}

func (r *RoomEntity) Save() (err error) {
	r.mux.Lock()
	defer r.mux.Unlock()
	err = GetCollection("rooms").Insert(r)
	return
}
