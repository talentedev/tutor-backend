package common

import (
	"net/http"

	"gitlab.com/learnt/api/pkg/core"

	"github.com/gin-gonic/gin"
)

// appHandler is a wrapper for the underlying AppURL function.
func appHandler(c *gin.Context) {
	query, ok := c.GetQuery("path")
	if !ok {
		query = "/"
	}

	url, err := core.AppURL(query)
	if err != nil {
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse("unknown environment"))
		return
	}

	c.String(http.StatusOK, url)
}

// apiHandler is a wrapper for the underlying APIURL function.
func apiHandler(c *gin.Context) {
	query, ok := c.GetQuery("path")
	if !ok {
		query = "/"
	}

	c.String(http.StatusOK, core.APIURL(query))
}

// SetupCore adds the core groupings to the router
func SetupCore(r *gin.RouterGroup) {
	r.GET("/app", appHandler)
	r.GET("/api", apiHandler)
}
