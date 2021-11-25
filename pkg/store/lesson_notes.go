package store

import (
	"time"

	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
)

type notesStore struct{}

func LessonNotesStore() *notesStore { return &notesStore{} }

func (ns *notesStore) ByLesson(lesson bson.ObjectId) ([]LessonNote, error) {
	notes := make([]LessonNote, 0)
	if err := GetCollection("lesson_notes").Find(bson.M{"lesson": lesson}).All(&notes); err != nil {
		return nil, errors.Wrap(err, "couldn't get notes")
	}

	return notes, nil
}

type LessonNote struct {
	ID bson.ObjectId `json:"id" bson:"_id"`

	Note string `json:"note" bson:"note"`

	Lesson bson.ObjectId `json:"lesson" bson:"lesson"`
	User   bson.ObjectId `json:"user" bson:"user"`

	CreatedAt time.Time  `json:"created_at" bson:"created_at"`
	DeletedAt *time.Time `json:"deleted_at" bson:"deleted_at"`
}

func (ln *LessonNote) Insert() error {
	if !ln.ID.Valid() {
		ln.ID = bson.NewObjectId()
	}

	if !ln.Lesson.Valid() {
		return errors.New("lesson is invalid")
	}

	if !ln.User.Valid() {
		return errors.New("user is invalid")
	}

	ln.CreatedAt = time.Now()

	if err := GetCollection("lesson_notes").Insert(ln); err != nil {
		return errors.Wrap(err, "couldn't insert note")
	}

	return nil
}
