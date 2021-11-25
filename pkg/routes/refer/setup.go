package refer

import (
	"time"

	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"

	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2/bson"
)

func getAffiliateLinkCount(userID bson.ObjectId) (int, error) {
	now := time.Now()
	pastWeek := now.AddDate(0, 0, -7)
	query := services.GetRefers().Find(bson.M{
		"referrer": userID,
		"created_at": bson.M{
			"$gte": pastWeek,
			"$lte": now,
		},
	})

	count, err := query.Count()
	if err != nil {
		return -1, err
	}

	return count, err
}

// Setup adds the refer routes to the router group provided.
func Setup(g *gin.RouterGroup) {
	g.GET("/exists/:code", referExistsHandler)
	g.POST("/invite", auth.Middleware, referInviteHandler)
	g.POST("/check", auth.Middleware, referCheckHandler)
}
