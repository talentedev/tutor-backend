package universities

import (
	"gitlab.com/learnt/api/pkg/store"

	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2/bson"
)

func Setup(g *gin.RouterGroup) {
	g.GET("", func(c *gin.Context) {
		name := c.Query("name")

		if name == "" {
			c.JSON(200, make([]string, 0))
			return
		}

		universities := make([]interface{}, 0)
		terms := bson.M{
			"name": bson.M{
				"$regex": bson.RegEx{Pattern: name, Options: "gi"},
			},
		}

		query := store.GetCollection("universities").Find(terms).Limit(10)
		query.All(&universities)
		c.JSON(200, universities)
	})
}
