package me

import (
	"time"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/ws"
	"gopkg.in/mgo.v2/bson"
)

type updateResponse struct {
	Message string `json:"message,omitempty"`
	Data    struct {
		Fields map[string]string `json:"fields,omitempty"`
		Raw    string            `json:"raw,omitempty"`
	} `json:"data,omitempty"`
	Error bool `json:"error,omitempty"`
}

func getAffiliateLinkCount(userID bson.ObjectId) (int, error) {
	now := time.Now()
	pastWeek := now.AddDate(0, 0, -7)
	query := services.GetRefers().Find(bson.M{
		"referrer": userID,
		"created_at": bson.M{
			"$gte": pastWeek,
			"$lte": now,
		},
	})

	count, err := query.Count()
	if err != nil {
		return -1, err
	}

	return count, err
}

// When a students opens booking panel a new item is added
// This is required to update booking data when tutor updates his profile
var bookingPending map[*ws.Connection]string = make(map[*ws.Connection]string, 0)

// Setup adds routes to the router
func Setup(g *gin.RouterGroup) {

	wsengine := ws.GetEngine()

	wsengine.OnLeave(func(c *ws.Connection) {
		delete(bookingPending, c)
	})

	wsengine.Listen("booking.pending", func(event ws.Event, engine *ws.Engine) {
		bookingPending[event.Source] = event.GetString("tutor")
	})

	wsengine.Listen("booking.cancel", func(event ws.Event, engine *ws.Engine) {
		delete(bookingPending, event.Source)
	})

	g.GET("", get)
	g.GET("/earnings", earnings)
	g.GET("/transactions", transactions)
	g.GET("/verify-email", verifyEmail)
	g.GET("/refer", referHandler)
	g.GET("/ics", icsHandler)
	g.GET("/lessons", affiliateLessonsHandler)
	g.GET("/calendar-lessons", getCalendarLessons)
	g.GET("/calendar-lessons/ics", getCalendarLessonsICS)
	g.GET("/calendar-lessons/dates", getCalendarLessonsDates)
	g.GET("/calendar-lessons/icsfeed", getCalendarLessonsICSFeed)
	g.PUT("", updateHandler)
	g.DELETE("", deleteAccount)
	g.PUT("/avatar", updateAvatar)
	g.PUT("/preferences", updatePreferences)
	g.POST("/telephone", updatePhone)
	g.PUT("/password", updatePassword)
	g.PUT("/instant", updateInstantStates)
	g.PUT("/payout", updatePayoutHandler)

	g.POST("/availability", createAvailability)
	g.PUT("/availability/:id", updateAvailability)
	g.DELETE("/availability/:id", removeAvailability)

	g.POST("/blackout", createBlackout)
	g.PUT("/blackout/:id", updateBlackout)
	g.DELETE("/blackout/:id", removeBlackout)

	g.POST("/cards", updatePaymentsCard)

	g.POST("/degrees", addDegree)
	g.DELETE("/degrees/:id", deleteDegree)

	g.POST("/subjects", addSubject)
	g.DELETE("/subjects/:id", deleteSubject)
	g.PUT("/subjects", updateSubject)

	g.POST("/add-favorite", addFavorite)
	g.DELETE("/remove-favorite/:id", removeFavorite)
	g.GET("/library", libraryHandler)
	g.POST("/library", libraryAddHandler)
	g.PUT("/library/:id", moveFileFromAttachmentHandler)
	g.DELETE("/library/:id", deleteFileHandler)
}

type profileUpdate byte

// Dispatch socket event for booking on user profile update
const (
	ProfileUpdateGeneric profileUpdate = 1 << iota
	ProfileUpdateSubject
	ProfileUpdateAvailability
)

func notifyProfileChangeFor(user *store.UserMgo, what profileUpdate) {
	for conn, tutor := range bookingPending {
		if tutor == user.ID.Hex() {
			conn.Send(ws.Event{
				Type: "tutor.profile.update",
				Data: ws.EventData{
					"tutor": tutor,
					"what":  what,
				},
			})
			break
		}
	}
}
