package projections

import "gopkg.in/mgo.v2/bson"

var TutorPublicProjection = bson.M{
	"$project": bson.M{
		"profile": bson.M{
			"first_name": 1,
			"last_name":  1,
			"about":      1,
			"avatar":     1,
		},
		"location": bson.M{
			"state": 1,
			"city":  1,
		},
		"tutoring":        1,
		"online":          1,
		"timezone":        1,
		"role":            1,
		"is_test_account": 1,
		"favorite":        1,
	},
}
