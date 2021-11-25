package core

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// CORS defines the CORS mechanism.
func CORS(c *gin.Context) {
	c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

	if c.Request.Method == http.MethodOptions {
		c.Status(200)
		c.Abort()
	}
}
