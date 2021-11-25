package oidc

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"golang.org/x/oauth2"
	goauth2 "google.golang.org/api/oauth2/v2"
	people "google.golang.org/api/people/v1"
	"io/ioutil"
	"net/http"
	"strings"
)

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

type OIDCResponse struct {
	Sub        string `json:"sub"`
	Name       string `json:"name"`
	Email      string `json:"email"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
	Picture    string `json:"picture"`
	Birthday   string `json:"birthday"`
	Phone      string `json:"phone"`
	Address    string `json:"address"`
}

func facebookCallback(c *gin.Context) {
	token, err := getAccessToken(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	src, err := core.TokenCache.Get(token)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	tok, err := src.(oauth2.TokenSource).Token()

	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}
	if token != tok.AccessToken {
		// token is renewed, return to the user to re-authenticate
		core.TokenCache.Delete(token)
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Unauthorized"))
		return
	}
	oauthConf := config.GetOAuthConfig("facebook")
	oauthConf.Scopes = []string{
		"email",
		"public_profile",
		"user_birthday",
	}
	client := oauthConf.Client(oauth2.NoContext, tok)
	response, err := client.Get(fmt.Sprintf("https://graph.facebook.com/me?access_token=%s&fields=email,first_name,last_name,name,birthday", tok.AccessToken))

	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	defer response.Body.Close()
	str, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	var fauser struct {
		Id        string
		Name      string
		Email     string
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Birthday  string `json:"birthday"`
		Photo     string `json:"profile_pic"`
	}

	err = json.Unmarshal([]byte(str), &fauser)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}
	resp := OIDCResponse{}
	resp.Sub = fauser.Id
	resp.Name = fauser.Name
	resp.Email = fauser.Email
	resp.GivenName = fauser.FirstName
	resp.FamilyName = fauser.LastName
	resp.Birthday = fauser.Birthday
	core.TokenCache.Delete(token)
	c.JSON(http.StatusOK, resp)
}

func googleCallback(c *gin.Context) {
	token, err := getAccessToken(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	src, err := core.TokenCache.Get(token)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	tok, err := src.(oauth2.TokenSource).Token()

	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}
	if token != tok.AccessToken {
		// token is renewed, return to the user to re-authenticate
		core.TokenCache.Delete(token)
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("Unauthorized"))
		return
	}

	oauthConf := config.GetOAuthConfig("google")
	oauthConf.Scopes = []string{
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
		"https://www.googleapis.com/auth/user.birthday.read",
	}
	client := oauthConf.Client(oauth2.NoContext, tok)
	service, err := goauth2.New(client)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}
	uService := goauth2.NewUserinfoService(service)
	gouser, err := uService.Get().Do()
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	resp := OIDCResponse{}
	resp.Sub = gouser.Id
	resp.Name = gouser.Name
	resp.Email = gouser.Email
	resp.GivenName = gouser.GivenName
	resp.FamilyName = gouser.FamilyName
	resp.Picture = gouser.Picture

	peopleservice, err := people.New(client)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}
	pService := people.NewPeopleService(peopleservice)
	goPeople, err := pService.Get(`people/me`).PersonFields("birthdays").Do()
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}
	if goPeople.Birthdays != nil && len(goPeople.Birthdays) >= 1 {
		d := goPeople.Birthdays[0].Date
		resp.Birthday = fmt.Sprintf("%d/%d/%d", d.Month, d.Day, d.Year)
	}
	goPeople, err = pService.Get(`people/me`).PersonFields("phoneNumbers").Do()
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	if goPeople.PhoneNumbers != nil && len(goPeople.PhoneNumbers) >= 1 {
		resp.Phone = goPeople.PhoneNumbers[0].Value
	}

	core.TokenCache.Delete(token)
	c.JSON(http.StatusOK, resp)
}

func Setup(g *gin.RouterGroup) {
	g.GET("/facebook", facebookCallback)
	g.GET("/google", googleCallback)
}
