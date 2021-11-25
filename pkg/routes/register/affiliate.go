package register

import (
	"net/http"
	"strings"
	"time"

	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type affiliateRegisterRequest struct {
	Email           string `json:"email" binding:"required"`
	FirstName       string `json:"first_name" binding:"required"`
	LastName        string `json:"last_name" binding:"required"`
	Telephone       string `json:"telephone"`
	Password        string `json:"password" binding:"required"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

func affiliateRegisterHandler(c *gin.Context) {
	req := affiliateRegisterRequest{}
	res := registerResponse{}
	res.Error.Fields = make(map[string]string)

	if err := c.BindJSON(&req); err != nil {
		for _, v := range strings.Split(err.Error(), "\n") {
			splitKey := strings.Split(v, "'")
			if len(splitKey) < 3 {
				continue
			}

			key := splitKey[3]
			res.Error.Fields[key] = "field is required"
		}

		res.Error.Message = errInvalidFields
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	if !utils.IsValidEmailAddress(req.Email) {
		res.Error.Fields["email"] = errEmailInvalid
		res.Error.Message = errInvalidFields
		c.JSON(http.StatusBadRequest, res)

		return
	}

	user, exists := services.NewUsers().ByEmail(req.Email)
	if exists || user != nil {
		res.Error.Message = errInvalidFields
		res.Error.Fields["email"] = errEmailTaken
		c.JSON(http.StatusBadRequest, res)

		return
	}

	now := time.Now()
	user = &store.UserMgo{
		Username: req.Email,
		Profile: store.Profile{
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Telephone: req.Telephone,
		},
		Emails: []store.RegisteredEmail{{
			Email:    req.Email,
			Created:  now,
			Verified: &now,
		}},
		Role:           store.RoleAffiliate,
		RegisteredDate: &now,
		Refer:          &store.Refer{},
		Preferences: &store.UserPreferences{
			ReceiveUpdates: true,
		},
		ApprovalStatus: store.ApprovalStatusApproved,
	}

	if err := user.SetReferralCode(false); err != nil {
		res.Error.Message = "can't set referral code"
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	if req.Password != req.ConfirmPassword {
		res.Error.Fields["confirm_password"] = "confirmation must match the password"
		res.Error.Message = errInvalidFields
		c.JSON(http.StatusBadRequest, res)

		return
	}

	if err := createPassword(req.Password, user); err != nil {
		res.Error.Fields["password"] = err.Error()
		res.Error.Message = errInvalidFields
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	if err := user.SaveNew(); err != nil {
		err = errors.Wrap(err, "can't create new user")
		res.Error.Message = "can't create new user"
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	p := services.GetPayments()
	if err := p.EnsureCustomer(user); err != nil {
		res.Error.Message = "can't create Stripe customer"
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	go sendMail(c, user, false)

	c.JSON(http.StatusOK, user.Dto())
}
