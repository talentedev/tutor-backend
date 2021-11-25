package bgcheck

import (
	"gopkg.in/mgo.v2/bson"

	"gitlab.com/learnt/api/pkg/bgcheck"
	"gitlab.com/learnt/api/pkg/store"
)

type handler struct {
	Users interface {
		ByID(bson.ObjectId) (*store.UserMgo, bool)
		ByCandidateID(string) (*store.UserMgo, error)
		SetBGCheckData(bson.ObjectId, *store.UserBGCheckData) error
	}

	API interface {
		CreateCandidate(*bgcheck.Candidate) (*bgcheck.Candidate, error)
		RetrieveReport(string) (*bgcheck.Report, error)
	}
}
