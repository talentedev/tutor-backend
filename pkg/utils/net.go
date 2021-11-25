package utils

import (
	"github.com/gin-gonic/gin"
)

// GetIP returns the X-Forwarded-For header from a handler's context.
func GetIP(c *gin.Context) string {
	return c.Request.Header.Get("X-Forwarded-For")
}
