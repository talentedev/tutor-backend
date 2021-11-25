package uploads

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/services"
	"gopkg.in/mgo.v2/bson"
)

func upload(c *gin.Context) {
	context := c.Request.FormValue("context")
	download := c.Request.FormValue("download")
	accept := c.Request.FormValue("accept")

	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("File missing from request"))
		return
	}

	if accept != "" {
		match := false
		for _, v := range strings.Split(accept, ",") {
			if strings.HasSuffix(strings.ToLower(header.Filename), strings.Trim(v, " ")) {
				match = true
			}
		}
		if !match {
			c.JSON(http.StatusBadRequest, core.NewErrorResponse("File extension doesn't match what was sent in."))
			return
		}
	}

	if context == "" {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Context missing"))
		return
	}

	upload, err := services.Uploads.Upload(nil, context, header.Filename, &file, download == "true")
	if err != nil {
		err = errors.Wrap(err, "couldn't upload file")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	c.JSON(201, upload)
}

func getUpload(c *gin.Context) {
	id := c.Query("id")
	if id == "" || !bson.IsObjectIdHex(id) {
		c.JSON(500, "Invalid request")
		return
	}
	upload, err := services.Uploads.Get(bson.ObjectIdHex(id))
	if err != nil {
		c.JSON(404, err)
		return
	}
	c.JSON(200, upload)
}

// Setup adds the routes to the router
func Setup(g *gin.RouterGroup) {
	g.POST("", upload)
	g.GET("", getUpload)
	g.StaticFS("", http.Dir(config.GetConfig().GetString("app.data")))
}
