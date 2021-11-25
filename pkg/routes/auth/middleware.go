package auth

import (
	"errors"
	"net/http"
	"strings"
	"time"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/store"

	jose "github.com/dvsekhvalnov/jose2go"
	"github.com/gin-gonic/gin"
	"gopkg.in/mgo.v2/bson"
)

func unauthorized(c *gin.Context, code int) {
	c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", code))
	c.Abort()
}

func getAccessToken(c *gin.Context) (token string, err error) {
	token = c.Query("access_token")

	if token == "" {
		authorization := c.Request.Header.Get("Authorization")

		if authorization == "" {
			return "", errors.New("no authorized token present")
		}

		if !strings.ContainsAny(authorization, "Bearer ") {
			return "", errors.New("invalid authorization header, bearer is missing")
		}

		split := strings.Split(authorization, "Bearer ")
		if len(split) < 2 {
			return "", errors.New("invalid authorization header, bearer is missing")
		}

		token = strings.Split(authorization, "Bearer ")[1]
	}

	return
}

func authMiddlewareFunc(abort bool) func(c *gin.Context) {
	return func(c *gin.Context) {
		token, err := getAccessToken(c)
		if err != nil {
			if abort {
				unauthorized(c, 100)
			}

			return
		}

		payload, headers, err := jose.Decode(token, []byte(config.GetConfig().GetString("security.token")))

		if err != nil {
			if abort {
				unauthorized(c, 101)
			}

			return
		}

		eat := time.Unix(int64(headers["eat"].(float64)), 0)

		if time.Now().After(eat) {
			if abort {
				unauthorized(c, 102)
			}

			return
		}

		if payload == "system" {
			c.Set("token", token)
			c.Set("user", &store.UserMgo{
				ID: "system",
				Profile: store.Profile{
					FirstName: "System",
					LastName:  "System",
				},
			})

			return
		}

		var user *store.UserMgo

		err = store.GetCollection("users").FindId(bson.ObjectIdHex(payload)).One(&user)
		if err != nil {
			if abort {
				unauthorized(c, 103)
			}

			return
		}

		c.Set("token", token)
		c.Set("user", user)
	}
}

func authMiddlewareResendFunc() func(c *gin.Context) {
	return func(c *gin.Context) {
		token, err := getAccessToken(c)
		if err != nil {
			unauthorized(c, 100)
		}

		payload, _, err := jose.Decode(token, []byte(config.GetConfig().GetString("security.token")))

		if err != nil {
			unauthorized(c, 101)
		}

		if payload == "system" {
			c.Set("token", token)
			c.Set("user", &store.UserMgo{
				ID: "system",
				Profile: store.Profile{
					FirstName: "System",
					LastName:  "System",
				},
			})

			return
		}

		var user *store.UserMgo

		err = store.GetCollection("users").FindId(bson.ObjectIdHex(payload)).One(&user)
		if err != nil {
			unauthorized(c, 103)
		}

		c.Set("token", token)
		c.Set("user", user)
	}
}

func Middleware(c *gin.Context) {
	authMiddlewareFunc(true)(c)
}

func MiddlewareSilent(c *gin.Context) {
	authMiddlewareFunc(false)(c)
}

func MiddlewareResend(c *gin.Context) {
	authMiddlewareResendFunc()(c)
}

func HasRoleMiddleware(role store.Role) func(c *gin.Context) {
	return func(c *gin.Context) {
		user, exist := store.GetUser(c)

		if !exist {
			unauthorized(c, 1000)
			return
		}

		if !user.HasRole(role) {
			unauthorized(c, 1001)
			return
		}
	}
}

func IsAdminMiddleware(c *gin.Context) {
	HasRoleMiddleware(store.RoleAdmin | store.RoleRoot)(c)
}

func IsAdmin(c *gin.Context) (yes bool) {
	user, exist := store.GetUser(c)

	if !exist {
		return
	}

	return user.HasRole(store.RoleAdmin)
}
