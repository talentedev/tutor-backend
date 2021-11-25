package importcontacts

import (
	"encoding/json"
	"net/http"
	"net/url"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var gmailConf *oauth2.Config
var gmailStates = make(map[string]string)

func init() {
	gmailURL, err := core.AppURL("/import/gmail")
	if err != nil {
		panic("unknown environment! check configuration!")
	}

	gmailConf = &oauth2.Config{
		RedirectURL: gmailURL,
		Scopes: []string{
			"https://www.googleapis.com/auth/contacts.readonly",
		},
		Endpoint: google.Endpoint,
	}
}

func gmailLinkHandler(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no user in context"))
		return
	}

	cfg := config.GetConfig()

	if gmailConf.ClientID == "" || gmailConf.ClientSecret == "" {
		gmailConf.ClientID = cfg.GetString("service.google.key")
		gmailConf.ClientSecret = cfg.GetString("service.google.secret")
	}

	state := utils.RandToken(32)
	link := gmailConf.AuthCodeURL(state)

	gmailStates[user.Username] = state

	c.String(http.StatusOK, link)
}

type gmailPeopleResponse struct {
	Connections []struct {
		Names []struct {
			DisplayName string `json:"displayName"`
			FamilyName  string `json:"familyName"`
			GivenName   string `json:"givenName"`
		} `json:"names"`
		Photos []struct {
			URL string `json:"url"`
		} `json:"photos"`
		EmailAddresses []struct {
			Value string `json:"value"`
		} `json:"emailAddresses"`
	} `json:"connections"`
	TotalPeople int `json:"totalPeople"`
	TotalItems  int `json:"totalItems"`

	Error struct {
		Code    int    `json:"code,omitempty"`
		Message string `json:"message,omitempty"`
		Status  string `json:"status,omitempty"`
	} `json:"error"`
}

func gmailStateHandler(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no user in context"))
		return
	}

	req := stateRequest{}
	if err := c.BindJSON(&req); err != nil {
		err = errors.Wrap(err, "invalid fields")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}

	initialState, ok := gmailStates[user.Username]
	if !ok {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no initial state for current user"))
		return
	}

	if req.State != initialState {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("received state different than initial"))
		return
	}

	token, err := gmailConf.Exchange(c, req.Code)
	if err != nil {
		err = errors.Wrap(err, "couldn't exchange code for token")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}

	client := gmailConf.Client(c, token)

	query := url.Values{}
	query.Add("personFields", "names,emailAddresses,photos")
	query.Add("pageSize", "2000")

	peopleRequest, err := client.Get("https://people.googleapis.com/v1/people/me/connections?" + query.Encode())
	if err != nil {
		err = errors.Wrap(err, "couldn't get people connections")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}
	defer peopleRequest.Body.Close()

	var peopleJSON *gmailPeopleResponse
	if err = json.NewDecoder(peopleRequest.Body).Decode(&peopleJSON); err != nil {
		err = errors.Wrap(err, "couldn't decode json")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}

	if peopleJSON.Error.Code != 0 && peopleJSON.Error.Message != "" {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(peopleJSON.Error.Message))
		return
	}

	res := peopleResponse{People: make([]person, 0)}

	for _, v := range peopleJSON.Connections {
		if len(v.EmailAddresses) == 0 {
			continue // no email address means we can't invite the person
		}

		var p person
		p.Email = v.EmailAddresses[0].Value

		if len(v.Names) > 0 {
			p.Name = v.Names[0].DisplayName
		}

		if len(v.Photos) > 0 {
			p.Avatar = v.Photos[0].URL
		}

		res.People = append(res.People, p)
	}

	res.Total = len(res.People)

	c.JSON(http.StatusOK, res)
}
