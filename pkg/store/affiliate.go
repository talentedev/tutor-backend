package store

import (
	"time"

	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
)

const (
	invitedTutor = iota + 1
	invitedStudent
)

// AffiliateLesson represents a completed lesson between a tutor & a student,
// when one of them is the referred user of an affiliate user.
type AffiliateLesson struct {
	ID        bson.ObjectId `json:"_id" bson:"_id"`
	CreatedAt time.Time     `json:"created_at" bson:"created_at"`
	// Affiliate is the ID of the affiliate user who referred the tutor or the student.
	Affiliate *bson.ObjectId `json:"affiliate" bson:"affiliate"`
	// Tutor is the ID of the tutor who completed the lesson.
	Tutor *bson.ObjectId `json:"tutor" bson:"tutor"`
	// Student is the ID of the student who completed the lesson.
	Student *bson.ObjectId `json:"student" bson:"student"`
	// Invited represents who was invited by the affiliate user.
	Invited int `json:"invited" bson:"invited"`
	// Transaction is the ID of the payment transaction between the tutor & the student.
	Transaction *bson.ObjectId `json:"transaction" bson:"transaction"`
	// Lesson is the ID of the completed lesson between the tutor & the student.
	Lesson *bson.ObjectId `json:"lesson" bson:"lesson"`
}

// Insert adds a new entry to the affiliate lessons collection.
func (al *AffiliateLesson) Insert() error {
	if !al.ID.Valid() {
		al.ID = bson.NewObjectId()
	}

	al.CreatedAt = time.Now()

	// check for an existing tutor & student
	var tutor, student *UserMgo
	if err := GetCollection("users").FindId(al.Tutor).One(&tutor); err != nil {
		err = errors.Wrap(err, "can't find tutor")
		return err
	}

	if err := GetCollection("users").FindId(al.Student).One(&student); err != nil {
		err = errors.Wrap(err, "can't find student")
		return err
	}

	// check who is the referral of the affiliate
	var referLink *ReferLink
	refers := GetCollection("refers")

	if err := refers.Find(bson.M{"referral": al.Tutor, "affiliate": true}).One(&referLink); err == nil {
		al.Affiliate = referLink.Referrer
		al.Invited = invitedTutor
	}

	if err := refers.Find(bson.M{"referral": al.Student, "affiliate": true}).One(&referLink); err == nil {
		al.Affiliate = referLink.Referrer
		al.Invited = invitedStudent
	}

	// check for an existing transaction in order to get payment status
	var transaction *TransactionMgo
	if err := GetCollection("transactions").FindId(al.Transaction).One(&transaction); err != nil {
		err = errors.Wrap(err, "can't find transaction")
		return err
	}

	return GetCollection("affiliate_lessons").Insert(al)
}
