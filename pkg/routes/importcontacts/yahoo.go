package importcontacts

import (
	"encoding/json"
	"net/http"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

var yahooConf *oauth2.Config
var yahooStates = make(map[string]string)

var yahooEndpoint = oauth2.Endpoint{
	AuthURL:  "https://api.login.yahoo.com/oauth2/request_auth",
	TokenURL: "https://api.login.yahoo.com/oauth2/get_token",
}

func init() {
	yahooURL, err := core.AppURL("/import/yahoo")
	if err != nil {
		panic("unknown environment! check configuration!")
	}

	yahooConf = &oauth2.Config{
		Endpoint:    yahooEndpoint,
		RedirectURL: yahooURL,
		Scopes:      []string{"sdct-r"},
	}
}

func yahooLinkHandler(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no user in context"))
		return
	}

	cfg := config.GetConfig()

	if yahooConf.ClientID == "" || yahooConf.ClientSecret == "" {
		yahooConf.ClientID = cfg.GetString("service.yahoo.key")
		yahooConf.ClientSecret = cfg.GetString("service.yahoo.secret")
	}

	state := utils.RandToken(32)
	link := yahooConf.AuthCodeURL(state)

	yahooStates[user.Username] = state

	c.String(http.StatusOK, link)
}

type yahooPeopleResponse struct {
	Contacts struct {
		Contact []struct {
			Fields []struct {
				Type  string      `json:"type"`
				Value interface{} `json:"value"`
			} `json:"fields"`
		} `json:"contact"`
		Total int `json:"total"`
	} `json:"contacts"`
}

func yahooStateHandler(c *gin.Context) {
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

	initialState, ok := yahooStates[user.Username]
	if !ok {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no initial state for current user"))
		return
	}

	if req.State != initialState {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("received state different than initial"))
		return
	}

	token, err := yahooConf.Exchange(c, req.Code)
	if err != nil {
		err = errors.Wrap(err, "couldn't exchange code for token")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}

	client := yahooConf.Client(c, token)

	peopleRequest, err := client.Get("https://social.yahooapis.com/v1/user/me/contacts?format=json&count=max")
	if err != nil {
		err = errors.Wrap(err, "couldn't get people connections")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}
	defer peopleRequest.Body.Close()

	var peopleJSON *yahooPeopleResponse
	if err = json.NewDecoder(peopleRequest.Body).Decode(&peopleJSON); err != nil {
		err = errors.Wrap(err, "couldn't decode json")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}

	res := peopleResponse{People: make([]person, 0), Total: peopleJSON.Contacts.Total}

	for _, c := range peopleJSON.Contacts.Contact {
		var p person

		for _, v := range c.Fields {
			switch v.Type {
			case "name":
				name := v.Value.(map[string]interface{})
				givenName := name["givenName"].(string)
				middleName := name["middleName"].(string)
				familyName := name["familyName"].(string)

				p.Name = givenName

				if middleName != "" {
					p.Name += " " + middleName
				}

				p.Name += " " + familyName
			case "email":
				p.Email = v.Value.(string)
			}
		}
		res.People = append(res.People, p)
	}

	c.JSON(http.StatusOK, res)
}
