package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/store"
)

// HasRole implements a middleware that checks if a user has the specified roles.
func HasRole(roles ...store.Role) func(c *gin.Context) {
	return func(c *gin.Context) {
		uParam, exist := c.Get("user")

		if !exist {
			c.Status(http.StatusUnauthorized)
			return
		}

		user := uParam.(store.UserMgo)

		if strings.IndexAny(c.Request.RequestURI, "/users") == 0 &&
			user.ID.Hex() == c.Param("user") {
			return // allow
		}

		for _, role := range roles {
			if !user.HasRole(role) {
				c.JSON(
					http.StatusUnauthorized,
					core.NewErrorResponse(
						"Your role is not unauthorized",
					),
				)
				c.Abort()
				break
			}
		}
	}
}
