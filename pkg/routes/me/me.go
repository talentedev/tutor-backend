package me

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"
	"time"

	"regexp"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/jinzhu/now"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/config"
	"gitlab.com/learnt/api/pkg/core"
	"gitlab.com/learnt/api/pkg/ics"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/pdf"
	"gitlab.com/learnt/api/pkg/routes/register"
	"gitlab.com/learnt/api/pkg/services"
	"gitlab.com/learnt/api/pkg/services/delivery"
	"gitlab.com/learnt/api/pkg/store"
	"gitlab.com/learnt/api/pkg/utils"
	m "gitlab.com/learnt/api/pkg/utils/messaging"
	"gitlab.com/learnt/api/pkg/utils/messaging/mail"
	"gopkg.in/mgo.v2/bson"
)

const defaultLibraryContext string = "library"

func get(c *gin.Context) {
	u, e := store.GetUser(c)
	if !e {
		c.JSON(http.StatusNotFound, nil)
		return
	}

	meDto := u.Dto(true)
	c.JSON(200, meDto)
}

type updateUserRequest struct {
	Profile     *store.Profile         `json:"profile"`
	Tutoring    *store.Tutoring        `json:"tutoring"`
	Email       string                 `json:"email"`
	Timezone    string                 `json:"timezone"`
	Location    *store.UserLocation    `json:"location"`
	Payments    map[string]interface{} `json:"payments"`
	Preferences *store.UserPreferences `json:"preferences"`
}

func deleteAccount(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}
	user.DisableAccount()
	c.Status(http.StatusGone)
}

func updateHandler(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	req := updateUserRequest{}
	res := updateResponse{}
	res.Data.Fields = make(map[string]string)

	if err := c.BindJSON(&req); err != nil {
		err = errors.Wrap(err, "invalid fields")
		res.Message = "invalid fields"
		res.Data.Raw = err.Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if req.Location != nil {
		if err := user.UpdateLocation(req.Location); err != nil {
			err = errors.Wrap(err, "couldn't update location")
			res.Message = "couldn't update location"
			res.Data.Raw = err.Error()
			c.JSON(http.StatusBadRequest, res)
			return
		}
	}

	if req.Profile != nil {
		if err := user.UpdateProfile(req.Profile); err != nil {
			err = errors.Wrap(err, "couldn't update profile")
			res.Message = "couldn't update profile"
			res.Data.Raw = err.Error()
			c.JSON(http.StatusInternalServerError, res)
			return
		}
	}

	if req.Preferences != nil {
		if err := user.UpdatePreferences(req.Preferences); err != nil {
			err = errors.Wrap(err, "couldn't update preferences")
			res.Message = "couldn't update preferences"
			res.Data.Raw = err.Error()
			c.JSON(http.StatusInternalServerError, res)
			return
		}
	}

	if req.Tutoring != nil && user.IsTutor() {
		if err := user.UpdateTutoring(req.Tutoring); err != nil {
			err = errors.Wrap(err, "couldn't update tutoring")
			res.Message = "couldn't update tutoring"
			res.Data.Raw = err.Error()
			c.JSON(http.StatusInternalServerError, res)
			return
		}
	}

	if req.Email != "" && !user.HasEmail(req.Email) {
		if err := user.AddEmail(req.Email); err != nil {
			err = errors.Wrap(err, "couldn't add new email")
			res.Message = "couldn't add new email"
			res.Data.Raw = err.Error()
			c.JSON(http.StatusInternalServerError, err)
			return
		}

		accessToken, err := user.GetAuthenticationToken(store.AuthScopeVerifyEmail)

		if err != nil {
			err = errors.Wrap(err, "couldn't get authentication token")
			res.Message = "couldn't get authentication token"
			res.Data.Raw = err.Error()
			c.JSON(http.StatusInternalServerError, res)
			return
		}
		d := delivery.New(config.GetConfig())
		go d.Send(user, m.TPL_VERIFY_EMAIL, &m.P{
			"VERIFY_EMAIL_URL": core.APIURL("/me/verify-email?access_token=%s&email=%s", accessToken, req.Email),
		})
	}

	if req.Timezone != "" {
		if err := user.UpdateTimezone(req.Timezone); err != nil {
			err = errors.Wrap(err, "couldn't update timezone")
			res.Message = "couldn't update timezone"
			res.Data.Raw = err.Error()
			c.JSON(http.StatusInternalServerError, res)
			return
		}
	}

	if req.Payments != nil {
		valid := true

		bankAccount := &services.BankAccountParams{}

		switch n := req.Payments["bank_account_name"].(type) {
		case string:
			bankAccount.BankAccountName = n
		}

		switch n := req.Payments["bank_account_type"].(type) {
		case string:
			bankAccount.BankAccountType = n
		}

		switch n := req.Payments["bank_account_number"].(type) {
		case string:
			if !utils.IsValidUSBankAccountNumber(n, false) {
				valid = false
				res.Data.Fields["bank_account_number"] = "invalid bank account number"
			} else {
				bankAccount.BankAccountNumber = n
			}
		default:
			valid = false
			res.Data.Fields["bank_account_number"] = "bank account number must be a string"
		}

		switch n := req.Payments["bank_account_routing"].(type) {
		case string:
			if !utils.IsValidUSBankRoutingNumber(n) {
				valid = false
				res.Data.Fields["bank_account_routing"] = "invalid bank account routing number"
			} else {
				bankAccount.BankAccountRouting = n
			}
		default:
			valid = false
			res.Data.Fields["bank_account_routing"] = "bank account routing number must be a string"
		}

		if !valid {
			res.Message = "invalid fields"
			c.JSON(http.StatusBadRequest, res)
			return
		}

		p := services.GetPayments()
		if _, err := p.SetBankAccount(user, bankAccount); err != nil {
			err = errors.Wrap(err, "couldn't create or update Stripe payment information")
			res.Message = "couldn't create or update Stripe payment information"
			res.Data.Raw = err.Error()
			c.JSON(http.StatusBadRequest, res)
			return
		}

	}

	go notifyProfileChangeFor(user, ProfileUpdateGeneric)

	// TODO: WS send tutor update profile

	c.JSON(http.StatusOK, user.Dto(true))
}

type updatePayoutRequest struct {
	EmployerIdentificationNumber string `json:"employer_identification_number"`
	SocialSecurityNumber         string `json:"social_security_number"`
}

func updatePayoutHandler(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	var req updatePayoutRequest
	var res updateResponse
	res.Data.Fields = make(map[string]string)

	if err := c.BindJSON(&req); err != nil {
		res.Error = true
		res.Message = "invalid fields"
		res.Data.Raw = err.Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if (req.EmployerIdentificationNumber != "" && req.SocialSecurityNumber != "") &&
		!utils.AreMutualExclusive(req.EmployerIdentificationNumber, req.SocialSecurityNumber) {
		res.Message = "invalid fields"
		res.Data.Fields["employer_identification_number"] = "EIN and SSN fields are mutual exclusive."
		res.Data.Fields["social_security_number"] = "EIN and SSN fields are mutual exclusive."
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if !utils.IsValidPattern(`^(|[0-9]{9})$`, req.EmployerIdentificationNumber) {
		res.Data.Fields["employer_identification_number"] = "invalid Employer Identification Number"
	}

	if !utils.IsSSN(req.SocialSecurityNumber) {
		if !utils.IsValidPattern(`^(###[- ]?##|\d{3}[- ]?\d{2})[- ]?\d{4}$`, req.SocialSecurityNumber) {
			res.Data.Fields["social_security_number"] = "invalid Social Security Number"
		}
	}

	if len(res.Data.Fields) > 0 {
		res.Message = "invalid fields"
		c.JSON(http.StatusBadRequest, res)
		return
	}

	err := user.UpdatePayoutData(req.EmployerIdentificationNumber, req.SocialSecurityNumber)
	if err != nil {
		res.Error = true
		res.Message = "couldn't update payout data"
		res.Data.Raw = err.Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if user.Payments == nil || user.Payments.ConnectID == "" {
		if err := register.CreateStripeConnectAccount(c, user, req.SocialSecurityNumber, user.Name(), req.EmployerIdentificationNumber); err != nil {
			res.Error = true
			res.Message = "Could not create payment account"
			res.Data.Raw = errors.Wrap(err, res.Message).Error()
			c.JSON(http.StatusBadRequest, res)
			return
		}
	}

	c.Status(http.StatusOK)
}

func addDegree(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	degree := store.TutoringDegree{}
	if err := c.BindJSON(&degree); err != nil {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	if user.HasDegree(degree.University, degree.Course) {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"Already have this degree",
			),
		)
		return
	}

	if degree.Certificate != nil && !services.Uploads.Valid(degree.Certificate) {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"Certificate upload does not exist. Try to upload again!",
			),
		)
		return
	}

	degree.ID = bson.NewObjectId()

	if err := user.AddDegree(degree); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
	}

	tutorProfileURL, err := core.AppURL("/admin/tutors/pending/%s", user.ID.Hex())
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	if err := mail.GetSender(config.GetConfig()).SendTo(m.HIRING_EMAIL, m.TPL_SUBJECT_EDUCATION_FOR_VERIFICATION, &m.P{
		"TUTOR_NAME":              user.Name(),
		"VERIFICATION_SUBMISSION": "EDUCATION",
		"PENDING_VERIFICATION":    tutorProfileURL,
	}); err != nil {
		c.JSON(http.StatusOK, core.NewErrorResponse(
			err.Error(),
		))
		return
	}
}

func deleteDegree(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	degreeID := c.Param("id")
	if !bson.IsObjectIdHex(degreeID) {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "invalid degree id"})
		return
	}

	if err := user.DeleteDegree(bson.ObjectIdHex(degreeID)); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "couldn't delete degree from database"})
		return
	}

	c.Status(http.StatusOK)
}

func addFavorite(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	favoriteTutor := store.FavoriteTutor{}
	if err := c.BindJSON(&favoriteTutor); err != nil {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	if user.IsAlreadyFavorite(favoriteTutor.Tutor) {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"Already a favorite",
			),
		)
		return
	}

	if err := user.AddFavorite(favoriteTutor); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
	}
}

func removeFavorite(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	favoriteID := c.Param("id")
	if !bson.IsObjectIdHex(favoriteID) {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "invalid favorite tutor id"})
		return
	}

	if err := user.RemoveFavorite(bson.ObjectIdHex(favoriteID)); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "couldn't remove favorite from database"})
		return
	}

	c.Status(http.StatusOK)
}

func addSubject(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}
	subject := store.TutoringSubject{}
	if err := c.BindJSON(&subject); err != nil {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	if user.HasSubject(subject.Subject) {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"Already have this subject",
			),
		)
		return
	}

	if subject.Certificate != nil && !services.Uploads.Valid(subject.Certificate) {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"Certificate upload does not exist. Try to upload again!",
			),
		)
		return
	}

	subject.ID = bson.NewObjectId()

	if err := user.AddSubject(subject); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
	}

	tutorProfileURL, err := core.AppURL("/admin/tutors/pending/%s", user.ID.Hex())
	if err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	if err := mail.GetSender(config.GetConfig()).SendTo(m.HIRING_EMAIL, m.TPL_SUBJECT_EDUCATION_FOR_VERIFICATION, &m.P{
		"TUTOR_NAME":              user.Name(),
		"VERIFICATION_SUBMISSION": "SUBJECT",
		"PENDING_VERIFICATION":    tutorProfileURL,
	}); err != nil {
		c.JSON(http.StatusOK, core.NewErrorResponse(
			err.Error(),
		))
		return
	}

	go notifyProfileChangeFor(user, ProfileUpdateSubject)
}

func updateSubject(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	subject := store.TutoringSubject{}
	if err := c.BindJSON(&subject); err != nil {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	if !user.HasSubject(subject.Subject) {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"No subject to update.",
			),
		)
		return
	}

	if subject.Certificate != nil && !services.Uploads.Valid(subject.Certificate) {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"Certificate upload does not exist. Try to upload again!",
			),
		)
		return
	}

	if err := user.UpdateSubject(subject); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
	}

	go notifyProfileChangeFor(user, ProfileUpdateSubject)

	c.Status(http.StatusOK)
}

func deleteSubject(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	subjectID := c.Param("id")
	if !bson.IsObjectIdHex(subjectID) {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "invalid subject id"})
		return
	}

	if err := user.DeleteSubject(bson.ObjectIdHex(subjectID)); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "couldn't delete subject from database"})
		return
	}

	go notifyProfileChangeFor(user, ProfileUpdateSubject)

	c.Status(http.StatusOK)
}

func earnings(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	amount := services.GetTransactions().GetAmount(user)

	c.Data(200, "text/plain", []byte(strconv.FormatFloat(amount, 'f', 2, 64)))
}

type transactionResponse struct {
	*store.TransactionDto
}

func transactions(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	from, err := time.Parse(time.RFC3339Nano, c.Query("from"))
	if err != nil {
		from = now.New(time.Now()).BeginningOfYear()
	}

	to, err := time.Parse(time.RFC3339Nano, c.Query("to"))
	if err != nil {
		to = now.New(time.Now()).EndOfYear()
	}

	var page int
	if p, err := strconv.ParseInt(c.Query("page"), 10, 32); err == nil {
		page = int(p)
	}

	var limit int
	if l, err := strconv.ParseInt(c.Query("limit"), 10, 32); err == nil {
		limit = int(l)
	}

	var count int
	var transactions []*store.TransactionDto
	if page != 0 && limit != 0 {
		count, transactions = services.GetTransactions().GetTransactionsPaged(user, from, to, page, limit)
		res := struct {
			Transactions []*store.TransactionDto `json:"transactions"`
			Count        int                     `json:"count"`
		}{}
		res.Transactions = transactions
		res.Count = count
		c.JSON(http.StatusOK, res)
		return
	} else {
		transactions = services.GetTransactions().GetTransactions(user, from, to)
	}

	transactionResponses := make([]*transactionResponse, len(transactions))
	for i, transaction := range transactions {
		transactionResponses[i] = &transactionResponse{transaction}

		loc, err := time.LoadLocation(user.Timezone)
		if err != nil {
			c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))
			return
		}

		transactionResponses[i].Time = transaction.Time.In(loc)
	}

	if c.Query("download") != "" {
		name := fmt.Sprintf("learnt-transactions-%s.pdf", from.Format("January-2006"))
		c.Header("Content-Type", "application/pdf")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", strings.ToLower(name)))
		c.Header("Content-Transfer-Encoding", "binary")
		c.Header("Accept-Ranges", "bytes")
	}

	if c.Query("serve") != "" || c.Query("download") != "" {
		if err := pdf.Transactions().Serve(c, user, transactions, from, to); err != nil {
			err = errors.Wrap(err, "failed to generate PDF file")
			c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))
		}
		return
	}
	c.JSON(http.StatusOK, transactionResponses)
}

func updatePhone(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	profile := new(store.Profile)

	if err := c.BindJSON(&profile); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	if err := user.UpdatePhone(profile); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}
}

func updatePreferences(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	preferences := new(store.UserPreferences)

	if err := c.BindJSON(&preferences); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	if preferences.ReceiveSMSUpdates {
		if user.GetPhoneNumber() == "" {
			c.JSON(
				http.StatusInternalServerError,
				core.NewErrorResponse(
					"empty phone number",
				),
			)
			return
		}
	}

	if err := user.UpdatePreferences(preferences); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}
}

const (
	invalidFieldsPassword  = "invalid fields"
	invalidRequestPassword = "invalid request"
	invalidPassword        = "invalid password"
)

type changePasswordRequest struct {
	Old     *string `json:"old,omitempty"`
	Token   *string `json:"token,omitempty"`
	New     string  `json:"new,omitempty"`
	Confirm string  `json:"confirm,omitempty"`
}

type changePasswordResponse struct {
	Error  string `json:"error,omitempty"`
	Fields struct {
		Token   string `json:"token,omitempty"`
		Old     string `json:"old,omitempty"`
		New     string `json:"new,omitempty"`
		Confirm string `json:"confirm,omitempty"`
	} `json:"fields"`
}

func updatePassword(c *gin.Context) {
	user, exists := store.GetUser(c)
	if !exists {
		return
	}

	res := changePasswordResponse{}

	req := changePasswordRequest{}
	if err := c.BindJSON(&req); err != nil {
		err = errors.Wrap(err, invalidFieldsPassword)
		res.Error = err.Error()
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if req.New == "" {
		res.Fields.New = "new password is required"
		res.Error = invalidFieldsPassword
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if req.Confirm == "" {
		res.Fields.Confirm = "new password confirmation is required"
		res.Error = invalidFieldsPassword
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if req.New != req.Confirm {
		res.Fields.Confirm = "new password confirmation must match the new password"
		res.Error = invalidFieldsPassword
		c.JSON(http.StatusBadRequest, res)
		return
	}

	const requirement = "password must contain at least a lowercase character, an uppercase character, and a digit"

	matchLower, _ := regexp.MatchString("[a-z]", req.New)
	matchUpper, _ := regexp.MatchString("[A-Z]", req.New)
	matchDigit, _ := regexp.MatchString("[0-9]", req.New)
	if !matchLower || !matchUpper || !matchDigit {
		res.Fields.New = requirement
		res.Error = invalidPassword
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if len(req.New) < 8 {
		res.Fields.New = "new password must be bigger than 8 characters"
		res.Error = invalidPassword
		c.JSON(http.StatusBadRequest, res)
		return
	}

	var verified bool

	// Verify by password
	if req.Old != nil {
		if !user.HasPassword(*req.Old, store.PasswordBcrypt) {
			res.Fields.Old = "old password is incorrect"
			res.Error = invalidFieldsPassword
			c.JSON(http.StatusBadRequest, res)
			return
		}
		verified = true
	}

	// Verify by token
	if req.Token != nil {
		_, tokenOwner, err := services.NewUsers().ParseAuthenticationToken(*req.Token)
		if tokenOwner.ID.Hex() != user.ID.Hex() || err != nil {
			res.Fields.Token = "invalid token"
			res.Error = invalidFieldsPassword
			c.JSON(http.StatusBadRequest, res)
			return
		}
		verified = true
	}

	if !verified {
		res.Error = invalidRequestPassword
		c.JSON(http.StatusBadRequest, res)
		return
	}

	if err := user.UpdatePassword(req.New, store.PasswordBcrypt); err != nil {
		err = errors.Wrap(err, "couldn't update password")
		res.Error = err.Error()
		c.JSON(http.StatusInternalServerError, res)
		return
	}
}

func verifyEmail(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	email := c.Query("email")

	if email == "" {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"Email is not provided",
			),
		)
		return
	}

	if !user.HasEmail(email) {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				"User does not have this email",
			),
		)
		return
	}

	if err := user.VerifyEmail(email); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				"Failed to set email as verified",
			),
		)
		return
	}

	if c.Query("redirect") != "" {
		redirectURL, err := url.Parse(c.Query("redirect"))

		if err != nil {
			c.JSON(
				http.StatusInternalServerError,
				core.NewErrorResponse(
					"Invalid redirect url",
				),
			)
			return
		}

		redirectURL.Query().Add("email", email)

		c.Redirect(http.StatusTemporaryRedirect, redirectURL.String())
	}
}

type updatePaymentsCardResponse struct {
	Error struct {
		Type    uint8  `json:"type,omitempty"`
		Message string `json:"message,omitempty"`
		Raw     string `json:"raw,omitempty"`
	} `json:"error"`
}

const (
	invalidFields uint8 = iota + 1
	invalidResponse
)

func updatePaymentsCard(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		logger.GetCtx(c).Error("User does not exist in updatePaymentsCard")
		return
	}

	res := updatePaymentsCardResponse{}

	service := services.GetPayments()

	body, err := ioutil.ReadAll(c.Request.Body)
	if err != nil {
		res.Error.Type = invalidFields
		res.Error.Message = "could not read request body"
		res.Error.Raw = err.Error()
		c.JSON(http.StatusBadRequest, res)
	}
	defer c.Request.Body.Close()

	tokenParams := services.TokenParams{}
	if err := binding.JSON.BindBody(body, &tokenParams); err != nil {
		card := services.CardParams{}
		if err := binding.JSON.BindBody(body, &card); err != nil {
			res.Error.Type = invalidFields
			res.Error.Message = "invalid fields provided"
			res.Error.Raw = err.Error()
			c.JSON(http.StatusBadRequest, res)
			return
		}

		tokenParams.Token, err = service.NewCardToken(user, card)
		if err != nil {
			res.Error.Message = "couldn't create card token for user account"
			res.Error.Raw = err.Error()
			res.Error.Type = invalidFields

			c.JSON(http.StatusBadRequest, res)
			return
		}
	}

	newcard, err := service.AddCard(user, tokenParams.Token)
	if err != nil {
		res.Error.Message = "couldn't add card to user account"
		res.Error.Raw = err.Error()
		res.Error.Type = invalidResponse

		c.JSON(http.StatusBadRequest, res)
		return
	}

	c.JSON(http.StatusOK, newcard)
}

func updateAvatar(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	upload := store.Upload{}

	if err := c.BindJSON(&upload); err != nil {
		c.JSON(
			http.StatusBadRequest,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	uploads := services.Uploads

	if err := uploads.Approve(&upload); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	var previous *store.Upload

	if user.Profile.Avatar != nil {
		previous = &store.Upload{
			ID:      user.Profile.Avatar.ID,
			Context: user.Profile.Avatar.Context,
		}
	}

	if err := user.SetAvatar(&upload); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	if previous != nil {
		go uploads.Delete(previous)
	}

	go func() {
		if err := notifyMessengerOnAvatarUpdate(c); err != nil {
			// TODO: ?
			logger.GetCtx(c).Errorf("Notify msg on avatar change error: %v", err)
		}
	}()
}

func notifyMessengerOnAvatarUpdate(c *gin.Context) (err error) {
	user, e := store.GetUser(c)
	if !e {
		return errors.New("user from this context does not exist")
	}

	userRaw := bson.M{
		"_id":       user.ID.Hex(),
		"firstName": user.Profile.FirstName,
		"lastName":  user.Profile.LastName,
		"avatar":    user.Avatar(),
	}

	data, err := json.Marshal(userRaw)

	token, tokenExist := c.Get("token")
	if !tokenExist {
		return errors.New("Token missing from request context")
	}

	endpoint := fmt.Sprint(config.GetConfig().GetString("messenger.address"), "/update")
	updateURL, _ := url.Parse(endpoint)

	params := url.Values{}
	params.Set("access_token", token.(string))
	updateURL.RawQuery = params.Encode()

	resp, err := http.Post(updateURL.String(), "application/json", bytes.NewBuffer(data))
	if err != nil {
		return errors.Wrap(err, fmt.Sprint("Fail to request ", updateURL.String()))
	}

	if resp.StatusCode != 200 {
		respBody, _ := ioutil.ReadAll(resp.Body)
		return errors.Errorf("Status code is %d while requesting %s with body %s",
			resp.StatusCode, updateURL.String(), respBody)
	}

	return
}

func updateInstantStates(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	r := make(map[string]bool)

	if err := c.BindJSON(&r); err != nil {
		c.JSON(
			http.StatusInternalServerError,
			core.NewErrorResponse(
				err.Error(),
			),
		)
		return
	}

	if v, e := r["session"]; e {
		err := store.GetCollection("users").UpdateId(
			user.ID,
			bson.M{
				"$set": bson.M{
					"tutoring.instant_session": v,
				},
			},
		)

		if err != nil {
			c.JSON(
				http.StatusInternalServerError,
				core.NewErrorResponse(
					err.Error(),
				),
			)
			return
		}
	}

	if v, e := r["booking"]; e {
		err := store.GetCollection("users").UpdateId(
			user.ID,
			bson.M{
				"$set": bson.M{
					"tutoring.instant_booking": v,
				},
			},
		)

		if err != nil {
			c.JSON(
				http.StatusInternalServerError,
				core.NewErrorResponse(
					err.Error(),
				),
			)
			return
		}
	}
}

type referLink struct {
	CreatedAt time.Time `json:"created_at"`
	Referral  string    `json:"referral,omitempty"`
	Email     string    `json:"email,omitempty"`
	Step      string    `json:"step"`
	Type      int       `json:"type"`
	Amount    float64   `json:"amount"`
	Completed bool      `json:"completed"`
}

type affiliate struct {
	Quota int `json:"quota,omitempty"`
}

type links struct {
	Total int         `json:"total"`
	Data  []referLink `json:"data"`
}

type referResponse struct {
	*store.Refer
	Affiliate *affiliate `json:"affiliate,omitempty"`
	Links     links      `json:"links"`
}

// referHandler offers refer data of the current user. Query params include page, a number,
// starting from 1, default 1; limit, a number, default 50.
func referHandler(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	// page for the pagination
	page, err := strconv.Atoi(c.Query("page"))
	if err != nil {
		page = 1
	}

	if page < 1 {
		page = 1
	}

	// limit for the pagination
	limit, err := strconv.Atoi(c.Query("limit"))
	if err != nil {
		limit = 50
	}

	// referral link type for filtering
	linkType, err := strconv.Atoi(c.Query("type"))
	if err != nil {
		linkType = 0
	}

	findQuery := bson.M{"referrer": user.ID, "disabled": false}

	switch linkType {
	case int(store.InvitedStep):
		findQuery["step"] = linkType
	case int(store.SignedUpStep):
		findQuery["$or"] = []bson.M{{"step": 2}, {"step": 3}}
	}

	query := services.GetRefers().Find(findQuery).Sort("-created_at")
	total, err := query.Count()
	if err != nil {
		err = errors.Wrap(err, "error counting query results")
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))
		return
	}

	var referLinks []*store.ReferLink
	err = query.Skip((page - 1) * limit).Limit(limit).All(&referLinks)
	if err != nil {
		err = errors.Wrap(err, "error getting refer data")
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))
		return
	}

	count, err := getAffiliateLinkCount(user.ID)
	if err != nil {
		err = errors.Wrap(err, "error getting affiliate link count")
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))
		return
	}

	res := referResponse{
		Refer: user.Refer,
		Links: links{Total: total, Data: make([]referLink, 0)},
	}

	if user.IsAffiliate() && count > -1 {
		res.Affiliate = &affiliate{Quota: 200 - count} // todo: hardcoded 200 max quota
	}

	for _, link := range referLinks {
		refLink := referLink{
			CreatedAt: link.CreatedAt,
			Email:     link.Email,
			Step:      link.Step.String(),
			Type:      int(link.Step),
			Amount:    link.Amount,
			Completed: link.Satisfied,
		}

		if link.Referral != nil {
			refLink.Referral = link.Referral.Hex()
		}

		res.Links.Data = append(res.Links.Data, refLink)
	}

	c.JSON(http.StatusOK, res)
}

type tokenResponse struct {
	Token string `json:"token"`
}

func icsHandler(c *gin.Context) {
	user, e := store.GetUser(c)
	if !e {
		return
	}

	accessToken, err := user.GetAuthenticationToken(store.AuthScopeVerifyEmail)

	res := tokenResponse{}

	if err != nil {
		c.JSON(http.StatusInternalServerError, res)
		return
	}

	res.Token = accessToken.AccessToken

	c.JSON(http.StatusOK, res)
}

func getCalendarLessonsICSFeed(c *gin.Context) {
	user, exists := store.GetUser(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, core.NewErrorResponse("Unauthorized"))
		return
	}

	lessons := services.NewUsers().GetLessons(user)
	if err := ics.Lessons().Serve(c, user, lessons); err != nil {
		err = errors.Wrap(err, "failed to generate ICS file")
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))
	}

	return
}

const (
	mb = 1 << 20
)

type sizer interface {
	Size() int64
}

func libraryAddHandler(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	if err := c.Request.ParseMultipartForm(50 * mb); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: err.Error()})
		return
	}

	context := fmt.Sprintf("%s/%s", defaultLibraryContext, user.ID.Hex())
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 50*mb)
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "File missing"})
		return
	}

	fileHeader := make([]byte, 512)
	// don't trust what the user says, check the header itself
	if _, err = file.Read(fileHeader); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: err.Error()})
		return
	}

	if _, err = file.Seek(0, 0); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: err.Error()})
		return
	}

	if file.(sizer).Size() > (50 * mb) {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "File is bigger than 50MB"})
		return
	}

	// TO-DO: do we consider duplicate files? file versioning? re-activate delete files and update it?
	upload, err := services.Uploads.UploadS3(user, context, header.Filename, &file, false)
	if err != nil {
		err = errors.Wrap(err, "couldn't upload file")
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: err.Error()})
		return
	}
	upload.AddedToLibrary = true

	f := &store.FilesMgo{
		ID:         upload.ID,
		Name:       upload.Name,
		Context:    upload.Context,
		URL:        upload.URL,
		Mime:       upload.Mime,
		Size:       upload.Size,
		Checksum:   upload.Checksum,
		UploadedBy: user.ID,
		CreatedAt:  time.Now(),
	}

	if err := f.SaveNew(); err != nil {
		err = errors.Wrap(err, "can't create new file")
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: err.Error()})
		return
	}

	if err := user.AddFile(upload); err != nil {
		c.JSON(http.StatusInternalServerError, updateResponse{Error: true, Message: err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func moveFileFromAttachmentHandler(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	fileId := c.Param("id")
	if !bson.IsObjectIdHex(fileId) {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "invalid file id"})
		return
	}

	id := bson.ObjectIdHex(fileId)

	if user.HasFile(id) {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "file already added"})
		return
	}
	// when a user decided to keep it in lib, retain it still, the other party may want to add to his own lib.
	upload, err := services.Uploads.Get(id)
	if err != nil {
		err = errors.Wrap(err, "file has expired.")
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: err.Error()})
		return
	}

	context := fmt.Sprintf("%s/%s", defaultLibraryContext, user.ID.Hex())

	if upload, err = services.Uploads.FetchAndMove(upload, context); err != nil {
		err = errors.Wrap(err, "couldn't move file")
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: err.Error()})
		return
	}
	upload.AddedToLibrary = true
	f := &store.FilesMgo{
		ID:         upload.ID,
		Name:       upload.Name,
		Context:    upload.Context,
		URL:        upload.URL,
		Mime:       upload.Mime,
		Size:       upload.Size,
		Checksum:   upload.Checksum,
		UploadedBy: upload.UploadedBy,
		CreatedAt:  time.Now(),
	}

	if err := f.SaveNew(); err != nil {
		err = errors.Wrap(err, "can't create new file")
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: err.Error()})
		return
	}

	if err := user.AddFile(upload); err != nil {
		c.JSON(http.StatusInternalServerError, updateResponse{Error: true, Message: err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func deleteFileHandler(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		return
	}

	fileID := c.Param("id")
	if !bson.IsObjectIdHex(fileID) {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "invalid file id"})
		return
	}

	if err := user.DeleteFile(bson.ObjectIdHex(fileID)); err != nil {
		c.JSON(http.StatusBadRequest, updateResponse{Error: true, Message: "couldn't delete file from database"})
		return
	}

	fstore := store.GetFilesStore()
	f, exists := fstore.Get(bson.ObjectIdHex(fileID))
	if exists || f != nil {
		f.DeleteFile()
	}

	if err := services.Uploads.RemoveFile(f.Context, f.ID); err != nil {
		c.JSON(http.StatusInternalServerError, updateResponse{Error: true, Message: "couldn't delete file."})
		return
	}

	c.Status(http.StatusOK)
}

type libraryResponse struct {
	Files []file `json:"files"`
}

type file struct {
	ID         string    `json:"_id"`
	Name       string    `json:"name"`
	URL        string    `json:"url"`
	Mime       string    `json:"mime" bson:"mime"`
	UploadedBy uploader  `json:"uploaded_by"`
	CreatedAt  time.Time `json:"created_at"`
}

type uploader struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func libraryHandler(c *gin.Context) {
	user, ok := store.GetUser(c)
	if !ok {
		c.String(http.StatusUnauthorized, "Unauthorized")
		return
	}

	res := libraryResponse{}

	l := services.GetLibrary()
	files, err := l.ActiveFilesByUser(user)

	if err != nil {
		c.JSON(http.StatusInternalServerError, core.NewErrorResponse(err.Error()))
		return
	}

	res.Files = make([]file, len(files))
	for i, f := range files {
		res.Files[i].ID = f.ID.Hex()
		res.Files[i].Name = f.Name
		res.Files[i].URL = f.URL
		res.Files[i].Mime = f.Mime
		res.Files[i].UploadedBy.FirstName = f.UploadedBy.Profile.FirstName
		res.Files[i].UploadedBy.LastName = f.UploadedBy.Profile.LastName
		res.Files[i].CreatedAt = f.CreatedAt
	}

	c.JSON(http.StatusOK, res)
}
