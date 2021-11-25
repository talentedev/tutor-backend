package subjects

import (
	"net/http"
	"strconv"

	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gopkg.in/mgo.v2"

	"gitlab.com/learnt/api/pkg/store"

	"regexp"

	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2/bson"
)

type response struct {
	Error   bool           `json:"error,omitempty"`
	Message string         `json:"message,omitempty"`
	Subject *store.Subject `json:"subject,omitempty"`
}

type createRequest struct {
	Subjects []string `json:"subjects" binding:"required"`
}

func listHandler(c *gin.Context) {
	subjects := make([]store.Subject, 0)
	var search = bson.M{}
	name := c.Query("name")

	_, err := regexp.Compile(name)
	if name != "" && err == nil {
		search = bson.M{"subject": bson.M{
			"$regex": bson.RegEx{
				Pattern: name,
				Options: "gi",
			},
		}}
	}

	query := store.GetCollection("subjects").Find(search)
	if c.Query("limit") != "" {
		limit, err := strconv.Atoi(c.Query("limit"))
		if err != nil {
			r := response{Error: true, Message: err.Error()}
			c.JSON(http.StatusBadRequest, r)
			return
		}
		query.Limit(limit)
	}

	if err := query.All(&subjects); err != nil {
		r := response{Error: true, Message: err.Error()}
		c.JSON(http.StatusBadRequest, r)
		return
	}

	c.JSON(http.StatusOK, subjects)
}

func searchHandler(c *gin.Context) {
	id := c.Param("id")

	if !bson.IsObjectIdHex(id) {
		// 12 byte hex
		r := response{
			Error:   true,
			Message: "invalid id",
		}
		c.JSON(http.StatusBadRequest, r)
		return
	}

	var subject *store.Subject
	if err := store.GetCollection("subjects").FindId(bson.ObjectIdHex(id)).One(&subject); err != nil {
		r := response{
			Error:   true,
			Message: err.Error(),
		}
		c.JSON(http.StatusBadRequest, r)
		return
	}

	r := response{Subject: subject}
	c.JSON(http.StatusOK, r)
}

func createHandler(c *gin.Context) {
	req := createRequest{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, nil)
		return
	}

	if err := store.CreateSubject(req.Subjects); err != nil {
		logger.GetCtx(c).Error(err)
		mgoError := err.(*mgo.LastError)
		if mgoError.Code == 11000 {
			c.JSON(http.StatusBadRequest, err.Error())
			return
		}
		c.JSON(http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, nil)
	return
}

func Setup(g *gin.RouterGroup) {
	g.GET("", listHandler)
	g.POST("", auth.Middleware, auth.IsAdminMiddleware, createHandler)
	g.GET("/search/:id", searchHandler)
}
