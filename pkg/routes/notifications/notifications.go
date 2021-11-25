package notifications

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/notifications"
	"gitlab.com/learnt/api/pkg/routes/auth"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

func get(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		err := core.NewErrorResponseWithCode("could not find user", http.StatusNotFound)
		core.HTTPError(c, err)
		return
	}

	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 10
	}

	offset, err := strconv.Atoi(c.Query("offset"))
	if err != nil {
		offset = 0
	}

	ns, err := notifications.ForUser(user, limit, offset)
	if err != nil {
		core.HTTPError(c, err)
		return
	}

	c.JSON(http.StatusOK, ns)
}

func notify(c *gin.Context) {
	req := notifications.NotifyRequest{}
	if err := c.BindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	response := <-notifications.Notify(&req)

	if !response.Succeed {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Failed to send notification"))
		return
	}
}

func deleteUserNotifications(c *gin.Context) {
	user, exist := store.GetUser(c)

	if !exist {
		return
	}

	filter := bson.M{"user": user.ID}

	if _, err := store.GetCollection("notifications").RemoveAll(filter); err != nil {
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse("Fail to delete notifications"))
		return
	}
}

func deleteNotification(c *gin.Context) {
	user, exist := store.GetUser(c)

	if !exist {
		return
	}

	if !bson.IsObjectIdHex(c.Param("id")) {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Invalid notification id"))
		return
	}

	id := bson.ObjectIdHex(c.Param("id"))

	var n notifications.NotificationMgo
	err := store.GetCollection("notifications").FindId(id).One(&n)

	if err != nil {
		c.JSON(http.StatusNotFound, core.NewErrorResponse("Notification not found"))
		return
	}

	if n.User.Hex() != user.ID.Hex() && !auth.IsAdmin(c) {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponse("Not authorized to delete this notification"))
		return
	}

	if err := store.GetCollection("notifications").RemoveId(id); err != nil {
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse("Fail to delete notification"))
		return
	}
}

// Setup adds the routes to the gin router
func Setup(g *gin.RouterGroup) {
	g.POST("", auth.IsAdminMiddleware, notify)
	g.GET("", auth.Middleware, get)
	g.DELETE("", auth.Middleware, deleteUserNotifications)
	g.DELETE("/:id", auth.Middleware, deleteNotification)
}
