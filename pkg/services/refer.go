package services

import (
	"fmt"
	"math"
	"time"

	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type refers struct {
	*mgo.Collection
}

// GetRefers returns the refers collection.
func GetRefers() *refers {
	return &refers{
		store.GetCollection("refers"),
	}
}

// ByReferrer returns all stored refer links.
func (r *refers) All() (referLinks []*store.ReferLink) {
	r.Find(bson.M{}).All(&referLinks)
	return
}

// ByReferrer returns all refer links with the specified referrer.
func (r *refers) ByReferrer(id bson.ObjectId) (referLinks []*store.ReferLink) {
	r.Find(bson.M{"referrer": id}).All(&referLinks)
	return
}

// ByReferral returns all refer links with the specified referral.
func (r *refers) ByReferral(id bson.ObjectId) (referLinks []*store.ReferLink) {
	r.Find(bson.M{"referral": id}).All(&referLinks)
	return
}

// NeedPayment returns all refer links that need payment to their referrers.
func (r *refers) NeedPayment() (referLinks []*store.ReferLink, err error) {
	err = r.Find(bson.M{"$or": []bson.M{
		// regular or affiliate referrers for signed up users
		{
			"step":      store.SignedUpStep,
			"satisfied": false,
		},
		// affiliate referrers for already completed links who joined in the past 90 days
		{
			"step":         store.CompletedStep,
			"affiliate":    true,
			"completed_at": bson.M{"$gte": time.Now().AddDate(0, 0, -90)},
		},
	}}).All(&referLinks)
	return
}

func (r *refers) GetReferrals(user *store.UserMgo, from, to time.Time) (referrals []*store.ReferralsDto) {
	c := store.GetCollection("transactions")

	pipe := c.Pipe([]bson.M{
		{
			"$match": bson.M{
				"referrer": user.ID,
				"created_at": bson.M{
					"$gte": from,
					"$lte": to,
				},
			},
		},
		{
			"$sort": bson.M{
				"created_at": -1,
			},
		},
		{
			"$project": bson.M{
				"_id":      1,
				"referrer": 1,
				"referral": 1,
				"step":     1,
			},
		},
	})

	var links []*store.ReferLink
	if err := pipe.All(&links); err != nil {
		logger.Get().Errorf("GetTransactions(): %v", err)
	}

	referrals = make([]*store.ReferralsDto, 0)
	for _, link := range links {
		if link.Referral != nil {
			if referral, ok := NewUsers().ByID(*link.Referral); ok {
				var isReferrerStudent bool
				var reward string
				ls := store.GetLessonsStore()
				lessons, _ := ls.GetAllUserLessons(referral)
				if user.IsStudentStrict() {
					isReferrerStudent = true
				}
				if isReferrerStudent {
					reward = "0"
					if len(lessons) > 1 {
						if !lessons[0].EndsAt.IsZero() {
							reward = "1"
						}
					}

					reward += " of 1 Lesson Completed"
				} else {
					var total int64
					for _, l := range lessons {
						total += int64(math.Floor(l.Duration().Hours()))
					}

					reward = fmt.Sprintf("%d of 10 Tutoring Hours Completed", total)
				}
				referrals = append(referrals, &store.ReferralsDto{link.ID, user.ToPublicDto(), link.Email, link.Step, reward})
			}
		} else {
			referrals = append(referrals, &store.ReferralsDto{link.ID, user.ToPublicDto(), link.Email, link.Step, ""})
		}
	}

	return
}
