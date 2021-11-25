package store

import (
	"gopkg.in/mgo.v2/bson"
)

func GetSubject(id bson.ObjectId) (subject *Subject, exist bool) {
	err := GetCollection("subjects").FindId(id).One(&subject)
	exist = err == nil
	return
}

func CreateSubject(names []string) (err error) {
	subjects := make([]interface{}, len(names))
	for i, name := range names {
		subjects[i] = Subject{
			ID: bson.NewObjectId(),
			Name: name,
		}
	}
	err = GetCollection("subjects").Insert(subjects...)
	return
}
