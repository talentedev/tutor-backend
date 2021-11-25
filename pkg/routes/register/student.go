package register

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/services/intercom"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"
	"gopkg.in/mgo.v2/bson"
)

func studentRegisterHandler(c *gin.Context) {
	req := userRegisterRequest{}
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

	req.Email = strings.ToLower(req.Email)
	var user *store.UserMgo
	var exists bool

	if !req.isSocial() {
		if !utils.IsValidEmailAddress(req.Email) {
			res.Error.Fields["email"] = errEmailInvalid
			res.Error.Message = errInvalidFields
			c.JSON(http.StatusBadRequest, res)

			return
		}
		user, exists = services.NewUsers().ByEmail(req.Email)
		if exists || user != nil {
			res.Error.Message = errInvalidFields
			res.Error.Fields["email"] = errEmailTaken
			c.JSON(http.StatusBadRequest, res)

			return
		}
	} else {
		user, exists = services.NewUsers().BySocialNetwork(*req.Network, *req.SocialID)
		if exists || user != nil {
			res.Error.Message = errSocialAlreadyRegistered
			c.JSON(http.StatusBadRequest, res)

			return
		}

		user, exists = services.NewUsers().ByUsername(req.Email)
		if exists || user != nil {
			res.Error.Message = errInvalidFields
			res.Error.Fields["email"] = errEmailTaken
			c.JSON(http.StatusBadRequest, res)

			return
		}
	}

	now := time.Now()
	user = &store.UserMgo{
		Username: req.Email,
		Profile: store.Profile{
			FirstName: req.FirstName,
			LastName:  req.LastName,
			Telephone: req.Telephone,
			Birthday:  &req.Birthday,
		},
		Emails: []store.RegisteredEmail{
			{
				Email:    req.Email,
				Created:  now,
				Verified: &now,
			},
		},
		Location:       req.Location,
		Role:           store.RoleStudent,
		RegisteredDate: &now,
		Refer:          &store.Refer{},
		Preferences: &store.UserPreferences{
			ReceiveUpdates: true,
		},
		ApprovalStatus: store.ApprovalStatusApproved,
	}

	if req.Location != nil && req.Location.Position != nil && req.Location.Position.Coordinates.Lat == 0 {
		user.Location.Position = nil
	}

	if err := user.SetReferralCode(false); err != nil {
		res.Error.Message = "can't set referral code"
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	if !req.isSocial() {
		if req.Password == nil {
			res.Error.Fields["password"] = "password is required"
			res.Error.Message = errInvalidFields
			c.JSON(http.StatusBadRequest, res)

			return
		}

		if *req.Password != req.ConfirmPassword {
			res.Error.Fields["confirm_password"] = "confirmation must match the password"
			res.Error.Message = errInvalidFields
			c.JSON(http.StatusBadRequest, res)

			return
		}

		if err := createPassword(*req.Password, user); err != nil {
			res.Error.Fields["password"] = err.Error()
			res.Error.Message = errInvalidFields
			res.Error.Data = err.Error()
			c.JSON(http.StatusBadRequest, res)

			return
		}
	} else {
		user.SocialNetworks = []store.SocialNetwork{
			{
				Network: *req.Network,
				Sub:     *req.SocialID,
				LastAccessToken: &store.AccessToken{
					Token: *req.AccessToken,
				},
			},
		}
	}

	if err := user.SaveNew(); err != nil {
		err = errors.Wrap(err, "can't create new user")
		res.Error.Message = "can't create new user"
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	go func() {
		contact := intercom.CreateContact(&intercom.Contact{
			Role:       intercom.RoleUser,
			Email:      user.GetEmail(),
			Phone:      user.GetPhoneNumber(),
			Name:       user.Name(),
			Avatar:     user.Avatar(),
			ExternalId: user.ID.Hex(),
		}, "students")
		if contact != nil {
			if err := services.NewUsers().UpdateId(user.ID, bson.M{"$set": bson.M{"intercom": &store.Intercom{
				ContactId:   contact.Id,
				WorkspaceId: contact.WorkspaceId,
			}}}); err != nil {
				logger.GetCtx(c).Errorf("Failed saving contact information of user %s", user.GetEmail())
			}

			tag := intercom.GetTag("Learnt student")
			if tag != nil {
				for _, t := range contact.Tags.Data {
					if t.Id == tag.Id {
						logger.GetCtx(c).Infof("Contact is already tagged")
						break
					}
				}

				tag = intercom.TagContact(tag.Id, contact.Id)
				if tag != nil {
					logger.GetCtx(c).Infof("Added tag %s to contact %s\n", tag.Name, contact.Id)
				}
			}
		}
	}()

	if err := handleReferAndCustomer(user, req); err != nil {
		logger.Get().Errorf("failed to create customer or referral: %w", err)
	}

	go sendMail(c, user, req.isSocial())

	c.JSON(http.StatusOK, user.Dto())
}
