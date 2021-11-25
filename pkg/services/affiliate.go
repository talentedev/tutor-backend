package services

import (
	"gitlab.com/learnt/api/pkg/store"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type affiliateLessons struct {
	*mgo.Collection
}

// GetAffiliateLessons returns the refers collection.
func GetAffiliateLessons() *affiliateLessons {
	return &affiliateLessons{
		store.GetCollection("affiliate_lessons"),
	}
}

// ByReferrer returns all stored affiliate Lessons.
func (al *affiliateLessons) All() (lessons []*store.AffiliateLesson, err error) {
	err = al.Find(bson.M{}).All(&lessons)
	return
}

// ByAffiliate returns all Lessons with the specified affiliate.
func (al *affiliateLessons) ByAffiliate(id bson.ObjectId) (lessons []*store.AffiliateLesson, err error) {
	err = al.Find(bson.M{"affiliate": id}).All(&lessons)
	return
}

// ByTutor returns all Lessons with the specified tutor.
func (al *affiliateLessons) ByTutor(id bson.ObjectId) (lessons []*store.AffiliateLesson, err error) {
	err = al.Find(bson.M{"tutor": id}).All(&lessons)
	return
}

// ByStudent returns all Lessons with the specified student.
func (al *affiliateLessons) ByStudent(id bson.ObjectId) (lessons []*store.AffiliateLesson, err error) {
	err = al.Find(bson.M{"student": id}).All(&lessons)
	return
}
