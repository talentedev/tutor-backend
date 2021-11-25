package refer

import (
	"net"
	"net/http"
	"net/mail"
	"net/url"
	"strings"
	"time"

	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/services/delivery"
	"gitlab.com/learnt/api/pkg/store"
	m "gitlab.com/learnt/api/pkg/utils/messaging"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
)

type referExistsResponse struct {
	Found bool `json:"found"`
}

// referExistsHandler checks if the referral code exists as a user's code. A false response
// means an invalid refer code. Does not require to be authenticated.
func referExistsHandler(c *gin.Context) {
	_, e := services.NewUsers().ByReferralCode(c.Param("code"))
	if !e {
		c.JSON(http.StatusOK, referExistsResponse{false})
	} else {
		c.JSON(http.StatusOK, referExistsResponse{true})
	}
}

type userInvite struct {
	Email string `json:"email"` // referral email address
	Name  string `json:"name"`  // referral name, if coming from contacts import
	Force bool   `json:"force"` // force the invite if there's a link created in the past 7 days
}

func (u userInvite) GetEmail() string {
	return u.Email
}

func (u userInvite) GetPhoneNumber() string {
	return ""
}

func (u userInvite) GetFirstName() string {
	return u.Name
}

func (u userInvite) IsReceiveUpdates() bool {
	return true
}

func (u userInvite) IsReceiveSMSUpdates() bool {
	return false
}

type referInviteRequest struct {
	Users []userInvite `json:"users"`
}

type inviteError struct {
	Email   string `json:"email"`
	Error   byte   `json:"error"`
	Message string `json:"message"`
}

type referInviteResponse struct {
	Invited []userInvite  `json:"invited"`
	Errors  []inviteError `json:"errors"`
}

const (
	_                    = iota
	errInvalidEmail byte = iota
	errUserRegistered
	errUserInvited
)

// check the validity of an email address
func checkEmailIsValid(email string) (ie *inviteError) {
	ie = new(inviteError)
	ie.Email = email

	// check for invalid email addresses
	var addrParser mail.AddressParser
	_, err := addrParser.Parse(email)

	if err != nil {
		ie.Error = errInvalidEmail
		ie.Message = errors.Wrap(err, "invalid email address: can't parse address").Error()

		return
	}

	// check for invalid domain
	split := strings.Split(email, "@")
	if len(split) != 2 {
		ie.Error = errInvalidEmail
		ie.Message = "invalid email address"

		return
	}

	_, err = net.LookupHost(split[1])

	if err != nil {
		ie.Error = errInvalidEmail
		ie.Message = "invalid email address: can't lookup host"

		return
	}

	return nil
}

// check an email address along with a user struct
func checkEmailLink(user *store.UserMgo, email string) (rl *store.ReferLink, ie *inviteError) {
	ie = new(inviteError)
	ie.Email = email

	// check for a registered user
	registeredUser, _ := services.NewUsers().ByEmail(email)
	if registeredUser != nil {
		ie.Error = errUserRegistered
		ie.Message = "user is already registered"

		return
	}

	// check if an email was sent in the previous week
	services.GetRefers().Find(bson.M{
		"referrer": user.ID, "email": email, "step": store.InvitedStep,
	}).Sort("-created_at").One(&rl)

	if rl != nil {
		weekSpan := time.Now().AddDate(0, 0, -7)
		if rl.CreatedAt.After(weekSpan) {
			ie.Error = errUserInvited
			ie.Message = "user already invited in the past 7 days"

			return
		}
	}

	return rl, nil
}

// referCheckHandler accepts a referInviteRequest{} and checks if there are any errors
// regarding invitation of the email address. Requires an authenticated user in order to get
// its referral links.
func referCheckHandler(c *gin.Context) {
	user, exists := store.GetUser(c)
	if !exists {
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("no user in context"))
		return
	}

	req := referInviteRequest{}
	if err := c.BindJSON(&req); err != nil {
		err = errors.Wrap(err, "invalid fields")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}

	res := struct {
		Valid []userInvite  `json:"valid"`
		Error []inviteError `json:"error"`
	}{
		Valid: make([]userInvite, 0),
		Error: make([]inviteError, 0),
	}

	for _, v := range req.Users {
		if inviteErr := checkEmailIsValid(v.Email); inviteErr != nil {
			res.Error = append(res.Error, *inviteErr)
			continue
		}

		if _, inviteErr := checkEmailLink(user, v.Email); inviteErr != nil {
			res.Error = append(res.Error, *inviteErr)
			continue
		}

		res.Valid = append(res.Valid, userInvite{Email: v.Email})
	}

	c.JSON(http.StatusOK, res)
}

// referInviteHandler accepts a list of email addresses and sends an email invitation to all of
// them. No custom link, sending the public link, but creating the refer link. Requires an
// authenticated user.
func referInviteHandler(c *gin.Context) {
	user, exists := store.GetUser(c)
	if !exists {
		return
	}

	count, err := getAffiliateLinkCount(user.ID)
	if err != nil {
		err = errors.Wrap(err, "error getting affiliate link count")
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))

		return
	}

	if user.IsAffiliate() && count > -1 && 200-count < 1 {
		// todo: hardcoded 200 max quota
		c.JSON(http.StatusBadRequest, core.NewErrorResponse("affiliate invite over the quota"))
		return
	}

	req := referInviteRequest{}
	if err := c.BindJSON(&req); err != nil {
		err = errors.Wrap(err, "invalid fields")
		c.JSON(http.StatusBadRequest, core.NewErrorResponse(err.Error()))

		return
	}

	res := referInviteResponse{
		Invited: make([]userInvite, 0),
		Errors:  make([]inviteError, 0),
	}

	for _, v := range req.Users {
		v.Email = strings.ToLower(v.Email)
		v.Email = strings.TrimSpace(v.Email)

		if inviteErr := checkEmailIsValid(v.Email); inviteErr != nil {
			res.Errors = append(res.Errors, *inviteErr)
			continue
		}

		referLink, inviteErr := checkEmailLink(user, v.Email)
		if inviteErr != nil {
			if inviteErr.Error == errUserInvited && v.Force {
				services.GetRefers().UpdateId(referLink.ID, bson.M{"$set": bson.M{"disabled": true}})
			} else {
				res.Errors = append(res.Errors, *inviteErr)
				continue
			}
		}

		referLink = &store.ReferLink{
			Referrer:  &user.ID,
			Email:     v.Email,
			Step:      store.InvitedStep,
			Affiliate: user.IsAffiliate(),
		}

		if err := referLink.Insert(); err != nil {
			res.Errors = append(res.Errors, inviteError{
				Email:   v.Email,
				Message: errors.Wrap(err, "couldn't create referral link").Error(),
			})

			continue
		}

		inviteLink, err := core.AppURL("/start/promo/%s?email=%s", user.Refer.ReferralCode, url.QueryEscape(v.Email))
		if err != nil {
			res.Errors = append(res.Errors, inviteError{
				Email:   v.Email,
				Message: errors.Wrap(err, "couldn't send invitation email").Error(),
			})

			continue
		}

		if v.Name == "" {
			v.Name = v.Email
		}
		d := delivery.New(config.GetConfig())
		err = d.Send(
			v,
			m.TPL_JOIN_INVITATION,
			&m.P{
				"SUBJECT":   core.T().Get("You've been invited to join Tutor App"),
				"FULL_NAME": user.Name(),
				"PROMO_URL": inviteLink,
			},
		)

		if err != nil {
			res.Errors = append(res.Errors, inviteError{
				Email:   v.Email,
				Message: errors.Wrap(err, "couldn't send invitation email").Error(),
			})

			continue
		}

		res.Invited = append(res.Invited, v)
	}

	c.JSON(http.StatusOK, res)
}
