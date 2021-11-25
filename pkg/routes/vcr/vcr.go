package vcr

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/ws"
)

func getsession(c *gin.Context) {

	room := services.VCRInstance().GetRoom(c.Param("room"))
	if room == nil {
		errResp(c, http.StatusNotFound, "Room not found")
		return
	}

	c.JSON(http.StatusOK, room.Whiteboard)
}

func create(c *gin.Context) {

	defer func() {
		if err := recover(); err != nil {
			errResp(c, 500, err.(string))
		}
	}()

	user, _ := store.GetUser(c)

	vcr := services.VCRInstance()

	room := vcr.GetRoom(c.Param("room"))
	if room == nil {
		errResp(c, http.StatusNotFound, "Room not found")
		return
	}

	obj, err := readObject(c)
	if err != nil {
		errResp(c, http.StatusInternalServerError, "Room not found")
		return
	}

	room.Whiteboard.Create(c.Param("session"), obj)

	room.Dispatch("vcr.wb.objects", ws.EventData{
		"room":    room.Room.ID.Hex(),
		"action":  "added",
		"session": c.Param("session"),
		"object":  obj.ID(),
	}, vcr.GetEngine().Hub.User(user.ID))
}

func get(c *gin.Context) {

	room := services.VCRInstance().GetRoom(c.Param("room"))
	if room == nil {
		errResp(c, http.StatusNotFound, "Room not found")
		return
	}

	o, e := room.Get(c.Param("session"), c.Param("object"))

	if e != nil {
		errResp(c, http.StatusNotFound, "Room not found")
		return
	}

	c.JSON(http.StatusOK, o)
}

func update(c *gin.Context) {

	room := services.VCRInstance().GetRoom(c.Param("room"))
	if room == nil {
		errResp(c, http.StatusNotFound, "Room not found")
		return
	}

	obj, err := readObject(c)
	if err != nil {
		errResp(c, http.StatusInternalServerError, "Room not found")
		return
	}

	if err := room.Whiteboard.Update(c.Param("session"), c.Param("object"), obj); err != nil {
		errResp(c, http.StatusBadRequest, err.Error())
		return
	}

	vcr := services.VCRInstance()

	user, _ := store.GetUser(c)

	room.Dispatch("vcr.wb.objects", ws.EventData{
		"room":    room.Room.ID.Hex(),
		"action":  "modified",
		"session": c.Param("session"),
		"object":  c.Param("object"),
	}, vcr.GetEngine().Hub.User(user.ID))

}

func delete(c *gin.Context) {

	room := services.VCRInstance().GetRoom(c.Param("room"))
	if room == nil {
		errResp(c, http.StatusNotFound, "Room not found")
		return
	}

	if err := room.Whiteboard.Remove(c.Param("session"), c.Param("object")); err != nil {
		errResp(c, http.StatusBadRequest, err.Error())
		return
	}

	vcr := services.VCRInstance()

	user, _ := store.GetUser(c)

	room.Dispatch("vcr.wb.objects", ws.EventData{
		"room":    room.Room.ID.Hex(),
		"action":  "removed",
		"session": c.Param("session"),
		"object":  c.Param("object"),
	}, vcr.GetEngine().Hub.User(user.ID))

	c.Status(http.StatusGone)
}

func errResp(c *gin.Context, status int, msg string) {
	c.JSON(
		http.StatusBadRequest,
		core.NewErrorResponse(msg),
	)
}

func readObject(c *gin.Context) (o services.CanvasObject, err error) {

	objs, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		errResp(c, http.StatusBadRequest, err.Error())
		return nil, err
	}

	obj := make(map[string]interface{}, 0)

	if err := json.Unmarshal(objs, &obj); err != nil {
		errResp(c, http.StatusBadRequest, err.Error())
		return nil, err
	}

	return obj, nil
}

func getRoomText(c *gin.Context) {

	room := services.VCRInstance().GetRoom(c.Param("room"))
	if room == nil {
		errResp(c, http.StatusNotFound, "Room not found")
		return
	}

	c.Data(200, "plain/text", []byte(room.Text))

}
func getRoomCode(c *gin.Context) {

	room := services.VCRInstance().GetRoom(c.Param("room"))
	if room == nil {
		errResp(c, http.StatusNotFound, "Room not found")
		return
	}

	c.JSON(200, room.Code)
}

func Setup(g *gin.RouterGroup) {

	g.GET("", func(c *gin.Context) {
		c.JSON(http.StatusOK, services.VCRInstance().GetRooms())
	})

	g.POST("/:room/whiteboard/:session", create)
	g.GET("/:room/whiteboard/:session/:object", get)
	g.PUT("/:room/whiteboard/:session/:object", update)
	g.DELETE("/:room/whiteboard/:session/:object", delete)
	g.GET("/:room/whiteboard-sessions", getsession)

	g.GET("/:room/text", getRoomText)
	g.GET("/:room/code", getRoomCode)

	g.GET("/:room/whiteboard", func(c *gin.Context) {

		for _, room := range services.VCRInstance().GetRooms() {
			if room.Room.ID.Hex() == c.Param("room") {
				c.JSON(http.StatusOK, room.Whiteboard)
				return
			}
		}

		c.Status(http.StatusNotFound)
	})

}
