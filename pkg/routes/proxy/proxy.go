package proxy

import (
	"github.com/gin-gonic/gin"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"golang.org/x/oauth2"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

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
		"public_profile",
	}

	tok, err := oauthConf.Exchange(oauth2.NoContext, code)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	baseUrl := oauthConf.RedirectURL + "#"
	params := url.Values{}
	params.Add("access_token", tok.AccessToken)
	params.Add("expires_in", strconv.Itoa(int(tok.Expiry.Unix()-time.Now().Unix())))
	params.Add("token_type", "bearer")
	params.Add("state", state)

	core.TokenCache.Set(tok.AccessToken, oauthConf.TokenSource(oauth2.NoContext, tok))

	c.Redirect(http.StatusTemporaryRedirect, baseUrl+params.Encode())
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
		"https://www.googleapis.com/auth/userinfo.profile",
	}
	tok, err := oauthConf.Exchange(oauth2.NoContext, code)
	if err != nil {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))
		return
	}

	baseUrl := oauthConf.RedirectURL + "#"
	params := url.Values{}
	params.Add("access_token", tok.AccessToken)
	params.Add("expires_in", strconv.Itoa(int(tok.Expiry.Unix()-time.Now().Unix())))
	params.Add("token_type", "bearer")
	params.Add("state", state)

	core.TokenCache.Set(tok.AccessToken, oauthConf.TokenSource(oauth2.NoContext, tok))

	c.Redirect(http.StatusTemporaryRedirect, baseUrl+params.Encode())
}

func Setup(g *gin.RouterGroup) {
	g.GET("/facebook", facebookCallback)
	g.GET("/google", googleCallback)
}
