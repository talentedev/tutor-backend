package store

import (
	"time"

	"gitlab.com/learnt/api/pkg/logger"
	"gopkg.in/mgo.v2/bson"
)

type TransactionDetails string

type transactionState byte

const (
	TransactionSent = iota
	TransactionApproved
	TransactionPending
	TransactionCancelled
)

type TransactionMgo struct {
	ID        bson.ObjectId    `json:"_id" bson:"_id"`
	User      bson.ObjectId    `json:"user" bson:"user"`
	Amount    float64          `json:"amount" bson:"amount"`
	Lesson    *bson.ObjectId   `json:"lesson" bson:"lesson"`
	Details   string           `json:"details" bson:"details"`
	Reference string           `json:"reference" bson:"reference"`
	Status    string           `json:"status" bson:"status"`
	State     transactionState `json:"state" bson:"state"`
	Time      time.Time        `json:"time" bson:"time"`
}

type TransactionDto struct {
	ID        bson.ObjectId    `json:"_id" bson:"_id"`
	User      *PublicUserDto   `json:"user" bson:"user"`
	Amount    float64          `json:"amount" bson:"amount"`
	Lesson    *LessonMgo       `json:"lesson" bson:"lesson"`
	Details   string           `json:"details" bson:"details"`
	Reference string           `json:"reference" bson:"reference"`
	Status    string           `json:"status" bson:"status"`
	State     transactionState `json:"state" bson:"state"`
	Time      time.Time        `json:"time" bson:"time"`
}

// Dto converts from the mongo version by filling in lesson and user
func (t *TransactionMgo) Dto() *TransactionDto {
	dto := &TransactionDto{
		ID:        t.ID,
		User:      &PublicUserDto{ID: t.User},
		Amount:    t.Amount,
		Details:   t.Details,
		Reference: t.Reference,
		Status:    t.Status,
		State:     t.State,
		Time:      t.Time,
	}

	var err error
	dto.User, err = getUserDto(t.User)
	if err != nil {
		logger.Get().Errorf("Could not get user in TransactionMgo Dto: %v", err)
	}

	if t.Lesson != nil {
		var exists bool
		dto.Lesson, exists = GetLessonsStore().Get(*t.Lesson)
		if !exists {
			logger.Get().Error("Could not get lesson in TransactionMgo Dto")
		}
	}
	return dto
}
