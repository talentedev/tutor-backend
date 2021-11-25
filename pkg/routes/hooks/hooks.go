package hooks

import (
	"gitlab.com/learnt/api/pkg/services"

	"github.com/gin-gonic/gin"
)

func Setup(g *gin.RouterGroup) {
	g.POST("/stripe", services.GetPayments().WebHook)
}
