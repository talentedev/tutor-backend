package core

import (
	"fmt"
	jose "github.com/dvsekhvalnov/jose2go"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/cache"
	"os"
	"time"
)

// AppVersion stores the current app version.
var AppVersion = "0.0.0"

// TokenCache is used for storing TokenSource. Temporary cache used by proxy and oidc
var TokenCache *cache.Cache = cache.New()

// IsDebugging returns whether the system environment DEBUG is set.
func IsDebugging() bool {
	if envDebug := os.Getenv("DEBUG"); envDebug != "" {
		return envDebug == "true"
	}

	return false
}

// AppURL returns a formatted URL with the correct URL based on environment.
func AppURL(endpoint string, a ...interface{}) (string, error) {
	var domain string
	env := config.GetConfig().GetString("app.env")

	if env == "" {
		return "", errors.New("unknown environment")
	}

	if env == "www" {
		domain = "https://learnt.io"
	}

	if env == "next" {
		domain = "https://next.learnt.io"
	}

	if env == "dev" || IsDebugging() {
		domain = "https://localhost:4200"
	}

	if endpoint[0] != '/' {
		endpoint = "/" + endpoint
	}

	return fmt.Sprintf(domain+endpoint, a...), nil
}

// APIURL returns a formatted URL with the correct API URL based on environment.
func APIURL(endpoint string, a ...interface{}) string {
	domain := "https://api.learnt.io"

	if envNext := os.Getenv("NEXT"); envNext != "" {
		domain = "https://next-api.learnt.io"
	}

	if IsDebugging() {
		domain = "http://localhost:5001"
	}

	if endpoint[0] != '/' {
		endpoint = "/" + endpoint
	}

	return fmt.Sprintf(domain+endpoint, a...)
}

// CreateAuthenticationToken creates a new JOSE token, set to expire in 24 hours from creation.
func CreateAuthenticationToken(payload string, secret string, scope string) (token string, err error) {
	iat := time.Now()
	eat := time.Now()

	eat = eat.Add(time.Hour * 24)

	issued := jose.Header("iat", iat.Unix())
	expire := jose.Header("eat", eat.Unix())
	secretHeader := jose.Header("secret", secret)
	scopeHeader := jose.Header("scope", scope)

	accessToken, err := jose.Sign(
		payload,
		jose.HS256,
		[]byte(config.GetConfig().GetString("security.token")),
		issued,
		expire,
		scopeHeader,
		secretHeader,
	)

	return accessToken, err
}
