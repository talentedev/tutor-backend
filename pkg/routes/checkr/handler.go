package checkr

import (
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

type handler struct {
	Users interface {
		ByCandidateID(string) (*store.UserMgo, error)
		SetCheckrData(bson.ObjectId, *store.UserCheckrData) error
	}
}
