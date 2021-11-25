package countries

import (
	"fmt"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/store"
	"net/http"

	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2/bson"
)

func countries(c *gin.Context) {
	var countries = make([]interface{}, 0)
	store.GetCollection("countries").Find(bson.M{}).All(&countries)
	c.JSON(200, countries)
}

func cities(c *gin.Context) {
	ids := c.Param("id")
	q := c.Query("q")

	if ids == "" || !bson.IsObjectIdHex(ids) {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				fmt.Sprintf("Invalid country id"),
			),
		)
		return
	}

	var cities = make([]interface{}, 0)

	store.GetCollection("cities").Pipe([]bson.M{
		{"$match": bson.M{
			"country": bson.ObjectIdHex(ids),
			"name":    bson.RegEx{Pattern: q, Options: "i"},
		}},
		{"$limit": 15},
		{"$project": bson.M{"name": 1}},
	}).All(&cities)

	c.JSON(200, cities)
}

func Setup(g *gin.RouterGroup) {
	g.GET("", countries)
	g.GET("/:id/cities", cities)
}
