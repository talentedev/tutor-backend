package register

import (
	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/utils"
	"net/http"
	"strings"
)

type verifyEmailRequest struct {
	Email string `json:"email" binding:"required"`
}

type verifyEmailResponse struct {
	Error struct {
		Fields map[string]string `json:"fields,omitempty"`

		Message string `json:"message,omitempty"`
		Data    string `json:"data,omitempty"`
	} `json:"error"`
}

func verifyEmailRegisterHandler(c *gin.Context) {
	req := verifyEmailRequest{}
	res := verifyEmailResponse{}
	res.Error.Fields = make(map[string]string)

	if err := c.ShouldBindJSON(&req); err != nil {
		for _, v := range strings.Split(err.Error(), "\n") {
			splitKey := strings.Split(v, "'")
			if len(splitKey) < 3 {
				continue
			}

			key := splitKey[3]
			res.Error.Fields[key] = "field is required"
		}

		err = errors.Wrap(err, "error binding json to struct")
		res.Error.Message = errInvalidFields
		res.Error.Data = err.Error()
		c.JSON(http.StatusOK, res)

		return
	}

	req.Email = strings.ToLower(req.Email)

	if !utils.IsValidEmailAddress(req.Email) {
		res.Error.Fields["email"] = errEmailInvalid
		res.Error.Message = errInvalidFields
		c.JSON(http.StatusOK, res)

		return
	}

	user, exists := services.NewUsers().ByEmail(req.Email)
	if exists || user != nil {
		res.Error.Message = errInvalidFields
		res.Error.Fields["email"] = errEmailTaken
		c.JSON(http.StatusOK, res)

		return
	}

	c.JSON(http.StatusOK, nil)
}
