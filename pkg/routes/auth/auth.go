package auth

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services/delivery"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"gopkg.in/mgo.v2/bson"
)

type GrantType string

var Password GrantType = "password"

type PasswordTokenRequest struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
	Remember bool   `json:"remember"`
}

func getTokenWithPassword(c *gin.Context) {
	r := PasswordTokenRequest{}
	err := c.BindJSON(&r)

	logger.GetCtx(c).Infof("token request: username %s password: *********; remember: %t", r.Username, r.Remember)

	if err != nil {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponse(err.Error()))

		return
	}

	r.Username = strings.ToLower(r.Username)

	query := bson.M{
		"username": r.Username,
		"approval": store.ApprovalStatusApproved,
		"disabled": false,
	}

	logger.GetCtx(c).Infof("Mongo query: %#v", query)

	var user *store.UserMgo
	if err = store.GetCollection("users").Find(query).One(&user); err != nil {
		logger.GetCtx(c).Errorf("user not found: %#v", user)
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 500))
		return
	}

	if user == nil {
		logger.GetCtx(c).Errorf("user is nil")
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 503))
		return
	}

	if user.Disabled {
		// provide backwards compatibility - if we search for disabled:false, we'll miss all accounts
		// with no disabled state
		logger.GetCtx(c).Errorf("user is disabled: %#v", user)
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 504))
		return
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Services.Password.Bcrypt), []byte(r.Password))
	if err != nil {
		logger.GetCtx(c).Errorf("Auth: %v", err)
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 501))

		return
	}

	eat := time.Now()
	eat = eat.Add(time.Minute * 60 * 24)

	token, err := user.GetAuthenticationToken(store.AuthScopeAuth)
	if err != nil {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 502))
		return
	}

	go func() {
		user.SetLoginDetails(c)
		if user.IsTutor() {
			data := make(map[string]string)
			data["id"] = user.ID.Hex()
			utils.Bus().Emit("TUTOR_LOGGED_IN", data)
		}
	}()

	c.JSON(200, token)
}

func googleCallback(c *gin.Context) {
	code := c.Query("code")

	state := c.Query("state")
	if state == "" {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("empty state"))
		return
	}

	oauthConf := config.GetOAuthConfig("google")
	oauthConf.Scopes = []string{
		"https://www.googleapis.com/auth/userinfo.email",
	}
	tok, err := oauthConf.Exchange(oauth2.NoContext, code)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	client := oauthConf.Client(oauth2.NoContext, tok)
	email, err := client.Get("https://www.googleapis.com/oauth2/v3/userinfo")
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}
	defer email.Body.Close()
	data, err := ioutil.ReadAll(email.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	var u struct {
		Sub string `json:"sub"`
	}

	if err = json.Unmarshal(data, &u); err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	query := bson.M{
		"approval": store.ApprovalStatusApproved,
		"disabled": false,
		"$and": []bson.M{
			{"social_networks.sub": u.Sub},
			{"social_networks.network": "google"},
		},
	}

	var user *store.UserMgo
	if err = store.GetCollection("users").Find(query).One(&user); err != nil {
		logger.GetCtx(c).Errorf("user not found: %#v", user)
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 500))
		return
	}

	if user == nil {
		logger.GetCtx(c).Errorf("user is nil")
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 503))
		return
	}

	if user.Disabled {
		// provide backwards compatibility - if we search for disabled:false, we'll miss all accounts
		// with no disabled state
		logger.GetCtx(c).Errorf("user is disabled: %#v", user)
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 504))
		return
	}

	eat := time.Now()
	eat = eat.Add(time.Minute * 60 * 24)

	token, err := user.GetAuthenticationToken(store.AuthScopeAuth)
	if err != nil {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 502))
		return
	}

	baseUrl := oauthConf.RedirectURL + "#"
	params := url.Values{}
	params.Add("access_token", token.AccessToken)
	params.Add("expires_in", strconv.Itoa(int(token.ExpiresIn)))
	params.Add("token_type", "bearer")
	params.Add("state", state)

	user.SetLastAccessToken("google", u.Sub, tok.AccessToken)
	go user.SetLoginDetails(c)

	c.Redirect(http.StatusTemporaryRedirect, baseUrl+params.Encode())
}

func facebookCallback(c *gin.Context) {
	code := c.Query("code")

	state := c.Query("state")
	if state == "" {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("empty state"))
		return
	}

	oauthConf := config.GetOAuthConfig("facebook")
	oauthConf.Scopes = []string{
		"email",
	}
	tok, err := oauthConf.Exchange(oauth2.NoContext, code)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	client := oauthConf.Client(oauth2.NoContext, tok)
	email, err := client.Get(fmt.Sprintf("https://graph.facebook.com/me?access_token=%s&fields=email", tok.AccessToken))
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}
	defer email.Body.Close()
	data, err := ioutil.ReadAll(email.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	var fauser struct {
		Id    string
		Email string
	}

	if err = json.Unmarshal(data, &fauser); err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}
	query := bson.M{
		"approval": store.ApprovalStatusApproved,
		"disabled": false,
		"$and": []bson.M{
			{"social_networks.sub": fauser.Id},
			{"social_networks.network": "facebook"},
		},
	}

	logger.GetCtx(c).Infof("Mongo query: %#v", query)

	var user *store.UserMgo
	if err = store.GetCollection("users").Find(query).One(&user); err != nil {
		logger.GetCtx(c).Errorf("user not found: %#v", user)
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 500))
		return
	}

	if user == nil {
		logger.GetCtx(c).Errorf("user is nil")
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 503))
		return
	}

	if user.Disabled {
		logger.GetCtx(c).Errorf("user is disabled: %#v", user)
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 504))
		return
	}

	eat := time.Now()
	eat = eat.Add(time.Minute * 60 * 24)

	token, err := user.GetAuthenticationToken(store.AuthScopeAuth)
	if err != nil {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 502))
		return
	}

	baseUrl := oauthConf.RedirectURL + "#"
	params := url.Values{}
	params.Add("access_token", token.AccessToken)
	params.Add("expires_in", strconv.Itoa(int(token.ExpiresIn)))
	params.Add("token_type", "bearer")
	params.Add("state", state)

	user.SetLastAccessToken("facebook", fauser.Id, tok.AccessToken)
	go user.SetLoginDetails(c)
	c.Redirect(http.StatusTemporaryRedirect, baseUrl+params.Encode())
}

func linkedinCallback(c *gin.Context) {
	code := c.Query("code")

	state := c.Query("state")
	if state == "" {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("empty state"))
		return
	}

	oauthConf := config.GetOAuthConfig("linkedin")
	oauthConf.Scopes = []string{
		"r_emailaddress",
	}
	tok, err := oauthConf.Exchange(oauth2.NoContext, code)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	client := oauthConf.Client(oauth2.NoContext, tok)
	req, err := http.NewRequest("GET", "https://api.linkedin.com/v2/emailAddress?q=members&projection=(elements*(handle~))", nil)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	req.Header.Set("Bearer", tok.AccessToken)
	response, err := client.Do(req)

	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	defer response.Body.Close()

	data, err := ioutil.ReadAll(response.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	var u map[string]interface{}

	if err = json.Unmarshal(data, &u); err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	if u == nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("empty response"))
		return
	}

	if _, ok := u["elements"]; !ok {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no elements"))
		return
	}

	if u["elements"] == nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("nil elements"))
		return
	}

	elements := u["elements"].([]interface{})
	if len(elements) > 1 {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("malformed elements"))
		return
	}

	if elements[0] == nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("nil elements"))
		return
	}

	element := elements[0].(map[string]interface{})
	if _, ok := element["handle~"]; !ok {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no handle"))
		return
	}

	if element["handle~"] == nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("malformed handle~"))
		return
	}

	handle := element["handle~"].(map[string]interface{})

	if _, ok := handle["emailAddress"]; !ok {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no emailAddress"))
		return
	}

	query := bson.M{
		"username": strings.ToLower(handle["emailAddress"].(string)),
		"approval": store.ApprovalStatusApproved,
		"disabled": false,
	}

	logger.GetCtx(c).Infof("Mongo query: %#v", query)

	var user *store.UserMgo
	if err = store.GetCollection("users").Find(query).One(&user); err != nil {
		logger.GetCtx(c).Errorf("user not found: %#v", user)
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 500))
		return
	}

	if user == nil {
		logger.GetCtx(c).Errorf("user is nil")
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 503))
		return
	}

	if user.Disabled {
		logger.GetCtx(c).Errorf("user is disabled: %#v", user)
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 504))
		return
	}

	eat := time.Now()
	eat = eat.Add(time.Minute * 60 * 24)

	token, err := user.GetAuthenticationToken(store.AuthScopeAuth)
	if err != nil {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponseWithCode("Unauthorized", 502))
		return
	}

	baseUrl := oauthConf.RedirectURL + "#"
	params := url.Values{}
	params.Add("access_token", token.AccessToken)
	params.Add("expires_in", strconv.Itoa(int(token.ExpiresIn)))
	params.Add("token_type", "bearer")
	params.Add("state", state)

	go user.SetLoginDetails(c)
	c.Redirect(http.StatusTemporaryRedirect, baseUrl+params.Encode())
}

func routeAuthMsg(c *gin.Context) {
	user, e := store.GetUser(c)

	if !e {
		return
	}

	dto := map[string]string{
		"_id":        user.ID.Hex(),
		"first_name": user.Profile.FirstName,
		"last_name":  user.Profile.LastName,
		"avatar":     user.Avatar(),
	}

	c.JSON(http.StatusOK, dto)
}

type recoverPasswordRequest struct {
	Email string `json:"email" binding:"required"`
}

func recoverPassword(c *gin.Context) {
	r := recoverPasswordRequest{}

	if err := c.BindJSON(&r); err != nil {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponse(err.Error()))
		return
	}

	logger.GetCtx(c).Infof("recover password for %s", r.Email)

	var user *store.UserMgo

	query := store.GetCollection("users").Find(bson.M{
		"emails.email": r.Email,
		// TODO: Check this
		// "emails.verified": true,
	})

	err := query.One(&user)

	if err != nil {
		logger.GetCtx(c).Errorf("Trying to recover a password for nonexistent email %s: %v", r.Email, err)

		return // silently
	}

	token, err := user.GetAuthenticationToken(store.AuthScopeForgotPassword)

	if err != nil {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponse("Failed to generate token"))
		return
	}

	cpURL, err := core.AppURL("/start/change-password?token=%s", token.AccessToken)
	if err != nil {
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse("unknown environment"))
		return
	}

	d := delivery.New(config.GetConfig())
	go d.Send(user, m.TPL_RECOVER_PASSWORD, &m.P{
		"CHANGE_PASSWORD_URL": cpURL,
	})
}

func Setup(g *gin.RouterGroup) {
	g.POST("", getTokenWithPassword)
	g.GET("/msg", Middleware, routeAuthMsg)
	g.POST("/recover", recoverPassword)
	g.GET("/google", googleCallback)
	g.GET("/facebook", facebookCallback)
	g.GET("/linkedin", linkedinCallback)
}
