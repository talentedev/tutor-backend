package importcontacts

import "github.com/gin-gonic/gin"

type stateRequest struct {
	State string `json:"state"`
	Code  string `json:"code"`
}

type person struct {
	Email  string `json:"email"`
	Name   string `json:"name"`
	Avatar string `json:"avatar"`
}

type peopleResponse struct {
	People []person `json:"people"`
	Total  int      `json:"total"`
}

func Setup(r *gin.RouterGroup) {
	r.GET("/gmail/link", gmailLinkHandler)
	r.POST("/gmail/state", gmailStateHandler)

	r.GET("/outlook/link", outlookLinkHandler)
	r.POST("/outlook/state", outlookStateHandler)

	r.GET("/yahoo/link", yahooLinkHandler)
	r.POST("/yahoo/state", yahooStateHandler)
}
