package register

import (
	"fmt"
	"regexp"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/services/delivery"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/utils/messaging/mail"
	"golang.org/x/crypto/bcrypt"
)

const (
	errCertificateUploadExpired = "one or more certificates need to be re-uploaded"
	errEmailInvalid             = "invalid email address"
	errEmailTaken               = "email address is already taken"
	errInvalidFields            = "invalid fields provided"
	errSocialAlreadyRegistered  = "already registered in selected Social Network"
	errResumeUploadExpired      = "resume needs to be re-uploaded"
	errTelephoneInvalid         = "invalid telephone number"
	errVideoUploadExpired       = "video needs to be re-uploaded"
)

type userSubject struct {
	Subject     string        `json:"subject"`
	Certificate *store.Upload `json:"certificate,omitempty" bson:"certificate"`
}

type userRegisterRequest struct {
	Referrer string `json:"referrer"`

	Email                string              `json:"email" binding:"required"`
	FirstName            string              `json:"first_name" binding:"required"`
	LastName             string              `json:"last_name" binding:"required"`
	Telephone            string              `json:"telephone"`
	Birthday             time.Time           `json:"birthday"`
	Location             *store.UserLocation `json:"location"`
	SocialSecurityNumber string              `json:"social_security_number"`

	Password        *string `json:"password"`
	ConfirmPassword string  `json:"confirm_password"`

	//Subjects []string      `json:"subjects"`
	Subjects            []userSubject `json:"subjects" bson:"subjects"`
	Video               *store.Upload `json:"video,omitempty"`
	YouTubeVideo        string        `json:"youtube_video,omitempty"`
	PromoteVideoAllowed bool          `json:"promote_video_allowed,omitempty"`
	Resume              *store.Upload `json:"resume"`

	//Not required but used if wanting a stipe account associated with a business
	CompanyName string  `json:"company_name"`
	CompanyEIN  string  `json:"company_ein"`
	SocialID    *string `json:"social_id"`
	Network     *string `json:"network"`
	AccessToken *string `json:"access_token"`
}

func (u userRegisterRequest) isSocial() bool {
	return u.Network != nil && u.SocialID != nil && u.AccessToken != nil
}

type registerResponse struct {
	Error struct {
		Fields map[string]string `json:"fields,omitempty"`

		Message string `json:"message,omitempty"`
		Data    string `json:"data,omitempty"`
	} `json:"error"`
}

func sendMail(c *gin.Context, user *store.UserMgo, isSocial bool) error {
	var template m.Tpl

	dashboardURL, err := core.AppURL("/login")
	if err != nil {
		return err
	}

	var data = m.P{
		"DASHBOARD_URL": dashboardURL,
	}

	if user.IsTutor() {
		template = m.TPL_REVIEWING_APPLICATION
		data["TUTOR_NAME"] = user.GetName()
	} else if user.IsAffiliate() {
		template = m.TPL_AFFILIATE_WELCOME
	} else {
		data["FIRST_NAME"] = user.GetFirstName()
		template = m.TPL_STUDENT_WELCOME
	}
	conf := config.GetConfig()
	d := delivery.New(conf)
	if err := d.Send(user, template, &data); err != nil {
		logger.GetCtx(c).Errorf("failed to send registration email: %v", err)
		return err
	}

	if template == m.TPL_REVIEWING_APPLICATION {
		if err := mail.GetSender(conf).SendTo(m.HIRING_EMAIL, m.TPL_NEW_APPLICATION_SUBMITTED, &data); err != nil {
			logger.GetCtx(c).Errorf("failed to send registration email: %v", err)
			return err
		}
	}

	return nil
}

func createPassword(password string, user *store.UserMgo) (err error) {
	const requirement = "password must contain at least a lowercase character, an uppercase character, and a digit"

	matchLower, _ := regexp.MatchString("[a-z]", password)
	matchUpper, _ := regexp.MatchString("[A-Z]", password)

	matchDigit, _ := regexp.MatchString("[0-9]", password)
	if !matchLower || !matchUpper || !matchDigit {
		return errors.New(requirement)
	}

	if len(password) < 8 {
		return errors.New("password must be bigger than 8 characters")
	}

	bcryptHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	secret := utils.RandBytes(16)
	user.Services = store.AuthorizationServices{
		Password: store.PasswordAuthorization{
			Bcrypt: string(bcryptHash),
		},
		Secret: fmt.Sprintf("%x", secret),
	}

	return
}

func handleReferAndCustomer(user *store.UserMgo, req userRegisterRequest) error {
	p := services.GetPayments()

	if err := p.EnsureCustomer(user); err != nil {
		return fmt.Errorf("failed to ensure customer for user %s: %w", user.ID, err)
	}

	link, err := user.SetReferrer(req.Referrer)
	if err != nil {
		return fmt.Errorf("failed to set referrer (%s) for user %s: %w", req.Referrer, user.ID, err)
	}

	referrer, ok := services.NewUsers().ByID(*link.Referrer)
	if !ok {
		return fmt.Errorf("failed to retrieve referrer with id %s for user %s: %w", *link.Referrer, user.ID, err)
	}

	if referrer.IsAffiliate() {
		// affiliates and non referred users get no credit
		var userType string

		if user.IsTutor() {
			userType = "tutor"
		} else {
			userType = "student"
		}
		d := delivery.New(config.GetConfig())
		go d.Send(referrer, m.TPL_AFFILIATE_USER_SIGNED_UP, &m.P{
			"REFERRED_USER": user.Name(),
			"USER_TYPE":     userType,
		})

		return nil
	}

	var referCredit float64

	switch {
	case user.IsTutorStrict():
		referCredit = 15
	case user.IsStudentStrict():
		referCredit = 15
	}

	bond := store.NoBond

	switch {
	case user.IsStudentStrict() && referrer.IsStudentStrict():
		bond = store.StudentToStudentBond
	case user.IsStudentStrict() && referrer.IsTutorStrict():
		bond = store.TutorToStudentBond
	case user.IsTutorStrict() && referrer.IsStudentStrict():
		bond = store.StudentToTutorBond
	case user.IsTutorStrict() && referrer.IsTutorStrict():
		bond = store.TutorToTutorBond
	}

	if err := link.SetBond(bond); err != nil {
		return fmt.Errorf("failed to set bond for referrer %s: %w", *link.Referrer, err)
	}

	if err := link.SetAmount(referCredit); err != nil {
		return fmt.Errorf("failed to set credit amount of %f for referrer %s: %w", referCredit, *link.Referrer, err)
	}
	return nil
}

// Function is unused. Deprecated?
// func transaction(user *store.UserMgo, amount float64, details string) (*store.TransactionMgo, error) {
// 	if user.IsStudentStrict() {
// 		amount = -amount
// 	}

// 	t := &store.TransactionMgo{
// 		User:    user.ID,
// 		Amount:  amount,
// 		Details: details,
// 	}

// 	return services.GetTransactions().New(t)
// }

// Setup adds the register routes to the router group provided.
func Setup(g *gin.RouterGroup) {
	g.POST("/tutor", tutorRegisterHandler)
	g.POST("/verifyEmail", verifyEmailRegisterHandler)
	g.POST("/student", studentRegisterHandler)
	g.POST("/affiliate", affiliateRegisterHandler)
}
