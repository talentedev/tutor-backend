package register

import (
	"net/http"
	"strings"
	"time"

	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/services/intercom"
	"gitlab.com/learnt/api/pkg/services/stripe"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"

	"github.com/gin-gonic/gin"
	"github.com/nyaruka/phonenumbers"
	"github.com/pkg/errors"
	"gopkg.in/mgo.v2/bson"
)

// isTutorApplyRequest validates Subjects, Resume, YouTubeVideo, and populates the fields on the UserMgo struct
func isTutorApplyRequest(request *userRegisterRequest, user *store.UserMgo) (err error) {
	if len(request.Subjects) == 0 {
		return errors.New("at least one subject is required for tutoring")
	}

	if request.Resume == nil {
		return errors.New("resume file is required when applying as a tutor")
	}

	subjects := make([]store.TutoringSubject, 0)

	for _, subject := range request.Subjects {
		subjects = append(subjects, store.TutoringSubject{
			ID:          bson.NewObjectId(),
			Subject:     bson.ObjectIdHex(subject.Subject),
			Certificate: subject.Certificate,
			Verified:    false,
		})
	}

	user.Tutoring = &store.Tutoring{
		Subjects:            subjects,
		Resume:              request.Resume,
		PromoteVideoAllowed: request.PromoteVideoAllowed,
	}

	if request.YouTubeVideo == "" && request.Video == nil {
		return errors.New("youtube link or video upload is required")
	}

	if request.Video != nil {
		user.Tutoring.Video = request.Video
	}
	if request.YouTubeVideo != "" {
		user.Tutoring.YouTubeVideo = request.YouTubeVideo
	}

	return
}

// tutorRegisterHandler validates payload and registers a tutor, returning a map of errors to the client if any are encountered
func tutorRegisterHandler(c *gin.Context) {
	req := userRegisterRequest{}
	res := registerResponse{}
	res.Error.Fields = make(map[string]string)

	// attempt to bind the the expected request object. if it fails, there are omitted or invalid fields
	if err := c.ShouldBindJSON(&req); err != nil {
		for _, v := range strings.Split(err.Error(), "\n") {
			splitKey := strings.Split(v, "'")
			if len(splitKey) < 3 {
				continue
			}

			key := splitKey[3]
			res.Error.Fields[key] = "field is required"
		}

		err = errors.Wrap(err, "error binding json to struct")
		res.Error.Message = errInvalidFields
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	// Confirm that the email or social network ID has not already been used. This does NOT register the user yet.
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

		user, exists := services.NewUsers().ByEmail(req.Email)
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

	phoneNumber, err := phonenumbers.Parse(req.Telephone, "US")
	if err != nil || !phonenumbers.IsValidNumber(phoneNumber) {
		res.Error.Fields["telephone"] = errTelephoneInvalid
		res.Error.Message = errInvalidFields
		c.JSON(http.StatusBadRequest, res)

		return
	}

	switch {
	case req.Birthday.IsZero():
		res.Error.Fields["birthday"] = "field is required"
		res.Error.Message = errInvalidFields
		c.JSON(http.StatusBadRequest, res)

		return
	case req.SocialSecurityNumber == "":
		res.Error.Fields["social_security_number"] = "field is required"
		res.Error.Message = errInvalidFields
		c.JSON(http.StatusBadRequest, res)

		return
	}

	now := time.Now()

	// Main fields are validated, so it's OK to start building the object as it will be stored in the DB
	user = &store.UserMgo{
		Username: req.Email,
		Profile: store.Profile{
			FirstName:            req.FirstName,
			LastName:             req.LastName,
			Telephone:            phonenumbers.Format(phoneNumber, phonenumbers.NATIONAL),
			Birthday:             &req.Birthday,
			SocialSecurityNumber: req.SocialSecurityNumber,
		},
		Location: req.Location,
		Emails: []store.RegisteredEmail{
			{
				Email:    req.Email,
				Created:  now,
				Verified: &now,
			},
		},
		Role:           store.RoleTutor,
		RegisteredDate: &now,
		Refer:          &store.Refer{},
		Tutoring: &store.Tutoring{
			Rate:         30,
			LessonBuffer: 15,
			Meet:         store.MeetOnline,
		},
		Preferences: &store.UserPreferences{
			ReceiveUpdates:    true,
			ReceiveSMSUpdates: true,
		},
		ApprovalStatus: store.ApprovalStatusNew,
	}

	// Don't store a position if it's not real
	if req.Location.Position != nil && req.Location.Position.Coordinates.Lat == 0 {
		user.Location.Position = nil
	}

	// Generate the referral code users will give others when referring them to learnt.io
	if err := user.SetReferralCode(false); err != nil {
		res.Error.Message = "can't set referral code"
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	// If the user is registering with a social network provider, store the token and ID, otherwise make sure the password is OK
	if !req.isSocial() {
		if req.Password != nil {
			if err := createPassword(*req.Password, user); err != nil {
				res.Error.Fields["password"] = err.Error()
				res.Error.Message = errInvalidFields
				res.Error.Data = err.Error()
				c.JSON(http.StatusBadRequest, res)

				return
			}
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

	// validate and populate Subjects, Resume, and Video/Youtube link
	if err := isTutorApplyRequest(&req, user); err != nil {
		res.Error.Message = errInvalidFields
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	//Check that all the associated uploaded files are valid for the applicable fields
	uploads := services.Uploads

	if req.Video != nil {
		if err := uploads.Approve(req.Video); err != nil {
			res.Error.Message = errVideoUploadExpired
			res.Error.Data = err.Error()
			c.JSON(http.StatusBadRequest, res)

			return
		}
	}

	if err := uploads.Approve(req.Resume); err != nil {
		res.Error.Message = errResumeUploadExpired
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	for _, sub := range req.Subjects {
		if sub.Certificate != nil {
			if err := uploads.Approve(sub.Certificate); err != nil {
				res.Error.Message = errCertificateUploadExpired
				res.Error.Data = err.Error()
				c.JSON(http.StatusBadRequest, res)
				return
			}
		}
	}

	// Execute the save to database
	if err := user.SaveNew(); err != nil {
		err = errors.Wrap(err, "can't create new user")
		res.Error.Message = "can't create new user"
		res.Error.Data = err.Error()
		c.JSON(http.StatusBadRequest, res)

		return
	}

	go intercom.UserCompletedApplication(user)

	if err := CreateStripeConnectAccount(c, user, req.SocialSecurityNumber, req.CompanyName, req.CompanyEIN); err != nil {
		// ignore error as the user account is already created
		logger.GetCtx(c).Errorf("Can't create payment account for user %s\nError: %v", user.Emails[0].Email, err)
	}

	if err := handleReferAndCustomer(user, req); err != nil {
		logger.GetCtx(c).Errorf("failed to handle referral and customer: %v", err)
	}

	//BUG error from sendmail is lost?
	go sendMail(c, user, req.isSocial())

	c.JSON(http.StatusOK, user.Dto(true))
}

// CreateStripeConnectAccount takes a user account and a few parameters to create a stripe connect account
func CreateStripeConnectAccount(c *gin.Context, user *store.UserMgo, ssn, companyName, ein string) error {
	email, err := user.MainEmail()
	if err != nil {
		return errors.Wrap(err, "could not get user's main email for stripe connect account")
	}

	if user.Location == nil {
		return errors.New("location cannot be empty when creating stripe connect account")
	}

	ssnLast4 := ssn[len(ssn)-4:]

	ap := stripe.AccountParams{
		TermsAccepted:     true,
		TermsAcceptanceIP: c.ClientIP(),
		Email:             email,
		FirstName:         user.Profile.FirstName,
		LastName:          user.Profile.LastName,
		Telephone:         user.Profile.Telephone,
		Birthday:          time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
		SSNLast4:          ssnLast4,
		CompanyName:       companyName,
		CompanyEIN:        ein,
		Address: stripe.Address{
			Address:    user.Location.Address,
			City:       user.Location.City,
			State:      user.Location.State,
			PostalCode: user.Location.PostalCode,
			Country:    user.Location.Country,
		},
		MetaData: map[string]string{stripe.MetadataLearntAccountID: user.ID.Hex()},
	}

	account, err := stripe.NewAccount(&ap)
	if err != nil {
		return err
	}

	if err := user.SetPaymentsConnect(account.ID); err != nil {
		return errors.Wrap(err, "could not add payment account to user")
	}

	return nil
}
