package importcontacts

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"
	"golang.org/x/oauth2"
)

var outlookConf *oauth2.Config
var outlookStates = make(map[string]string)

var outlookEndpoint = oauth2.Endpoint{
	AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
	TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
}

func init() {
	outlookURL, err := core.AppURL("/import/outlook")
	if err != nil {
		panic("unknown environment! check configuration!")
	}

	outlookConf = &oauth2.Config{
		RedirectURL: outlookURL,
		Scopes: []string{
			"https://outlook.office.com/contacts.read",
		},
		Endpoint: outlookEndpoint,
	}
}

func outlookLinkHandler(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no user in context"))
		return
	}

	cfg := config.GetConfig()

	if outlookConf.ClientID == "" || outlookConf.ClientSecret == "" {
		outlookConf.ClientID = cfg.GetString("service.outlook.key")
		outlookConf.ClientSecret = cfg.GetString("service.outlook.secret")
	}

	state := utils.RandToken(32)
	link := outlookConf.AuthCodeURL(state)

	outlookStates[user.Username] = state

	c.String(http.StatusOK, link)
}

type outlookPeopleResponse struct {
	Value []struct {
		DisplayName    string `json:"DisplayName"`
		Surname        string `json:"Surname"`
		GivenName      string `json:"GivenName"`
		EmailAddresses []struct {
			Address string `json:"Address"`
		} `json:"EmailAddresses"`
	} `json:"value"`
}

func outlookStateHandler(c *gin.Context) {
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

	initialState, ok := outlookStates[user.Username]
	if !ok {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no initial state for current user"))
		return
	}

	if req.State != initialState {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("received state different than initial"))
		return
	}

	token, err := outlookConf.Exchange(c, req.Code)
	if err != nil {
		err = errors.Wrap(err, "couldn't exchange code for token")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}

	client := outlookConf.Client(c, token)

	peopleRequest, err := client.Get("https://outlook.office.com/api/v2.0/me/contacts")
	if err != nil {
		err = errors.Wrap(err, "couldn't get people connections")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}
	defer peopleRequest.Body.Close()

	var peopleJSON *outlookPeopleResponse
	if err = json.NewDecoder(peopleRequest.Body).Decode(&peopleJSON); err != nil {
		err = errors.Wrap(err, "couldn't decode json")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}

	res := peopleResponse{People: make([]person, 0)}

	for _, v := range peopleJSON.Value {
		if len(v.EmailAddresses) == 0 {
			continue
		}

		res.People = append(res.People, person{
			Email: v.EmailAddresses[0].Address,
			Name:  v.DisplayName,
		})
	}

	res.Total = len(res.People)

	c.JSON(http.StatusOK, res)
}
