package bgcheck

import (
	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/bgcheck"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/services"
)

// Setup adds all the routes to the router
func Setup(g *gin.RouterGroup) {
	// wire up dependencies for the handlers
	handler := &handler{
		Users: services.NewUsers(),
		API:   bgcheck.New(),
	}

	g.POST("/candidate", auth.Middleware, auth.IsAdminMiddleware, handler.candidateCreateHandler)
	g.GET("/candidate/:id", auth.Middleware, auth.IsAdminMiddleware, handler.candidateGetHandler)
	g.GET("/report/:id", auth.Middleware, auth.IsAdminMiddleware, handler.reportGetHandler)

	g.POST("/webhook", handler.webhookHandler)
}
