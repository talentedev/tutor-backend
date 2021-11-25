package services

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	stripeGo "github.com/stripe/stripe-go"
	"gitlab.com/learnt/api/pkg/logger"
	"gitlab.com/learnt/api/pkg/services/models"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
	"gitlab.com/learnt/api/pkg/services/stripe"
	"gitlab.com/learnt/api/pkg/store"
	"gopkg.in/mgo.v2/bson"
)

const (
	platformFeePercentage = 0.3
	prefix                = "Learnt Lesson"
	instantPrefix         = "Learnt Instant Lesson"
)

// CardParams are params needed to create a new credit card if you don't pass a token
type CardParams struct {
	Name     string `json:"name" binding:"required"`
	Number   string `json:"number" binding:"required"`
	Month    string `json:"month" binding:"required"`
	Year     string `json:"year" binding:"required"`
	CVC      string `json:"cvc" binding:"required"`
	Currency string `json:"currency"`
}

// BankAccountParams are the params need to update a bank account
type BankAccountParams struct {
	Token              string `json:"token,omitempty"`
	BankAccountName    string `json:"bank_account_name"`
	BankAccountType    string `json:"bank_account_type"`
	BankAccountNumber  string `json:"bank_account_number"`
	BankAccountRouting string `json:"bank_account_routing"`
}

// TokenParams allows sending a token instead of sensitive information
type TokenParams struct {
	Token string `json:"token" binding:"required"`
}

type CreditParams struct {
	Amount int64  `json:"amount"`
	Reason string `json:"reason"`
	Notes  string `json:"notes"`
}

type payments struct {
	hooks map[WebHookEventType]func(event *WebHookEvent)
}

// GetPayments returns an configured struct that holds functions for working with payments
func GetPayments() *payments {
	return &payments{}
}

// WebHookEventType are type events
type WebHookEventType string

const (
	accountUpdated              WebHookEventType = "account.updated"
	customerSubscriptionUpdated WebHookEventType = "customer.subscription.updated"
	invoicePaymentSucceeded     WebHookEventType = "invoice.payment_succeeded"
)

// WebHookEvent is a stripe wehbook https://stripe.com/docs/api/webhook_endpoints
type WebHookEvent struct {
	ID      string           `json:"id"`
	Type    WebHookEventType `json:"type"`
	Object  string           `json:"object"`
	Created uint64           `json:"created"`
	Data    interface{}      `json:"data"`
}

func (p *payments) Init(ctx context.Context) {
	stripe.Init()
	p.hooks = make(map[WebHookEventType]func(event *WebHookEvent))

	p.hooks[customerSubscriptionUpdated] = p.HookCustomerSubscriptionUpdated
	p.hooks[invoicePaymentSucceeded] = p.HookInvoicePaymentSucceeded
}

func getSentTransactions(user *store.UserMgo) ([]*store.TransactionMgo, error) {
	var t []*store.TransactionMgo

	tr := GetTransactions()
	if t == nil {
		return t, errors.New("couldn't get transactions collection")
	}

	err := tr.Find(bson.M{
		"user":  user.ID,
		"state": store.TransactionSent,
	}).All(&t)

	return t, errors.Wrap(err, "couldn't get transactions for user "+user.ID.Hex())
}

func clearTransactions(user *store.UserMgo) error {
	return GetTransactions().Update(bson.M{
		"user":  user.ID,
		"state": store.TransactionSent,
	}, bson.M{
		"state": store.TransactionApproved,
	})
}

func (p *payments) HookCustomerSubscriptionUpdated(event *WebHookEvent) {
	logger.Get().Info("HookCustomerSubscriptionUpdated")
}

func (p *payments) HookInvoicePaymentSucceeded(event *WebHookEvent) {
	logger.Get().Info("HookInvoicePaymentSucceeded")
}

func (p *payments) WebHook(c *gin.Context) {
	event := WebHookEvent{}

	if err := c.BindJSON(&event); err != nil {
		logger.Get().Errorf("Fail to bind stripe webhook event data: %v", err)
		return
	}

	if fn, exist := p.hooks[event.Type]; exist {
		fn(&event)
	} else {
		logger.Get().Errorf("No hooks for: %v", event.Type)
	}
}

func (p *payments) EnsureCustomer(user *store.UserMgo) (err error) {
	if user.Payments == nil || user.Payments.CustomerID == "" {
		return p.NewCustomer(user)
	}
	return nil
}

func (p *payments) NewCardToken(user *store.UserMgo, params CardParams) (string, error) {
	if err := p.EnsureCustomer(user); err != nil {
		return "", errors.Wrapf(err, "couldn't ensure customer to add create card token (userID:%s)", user.ID.Hex())
	}
	token, err := stripe.NewCardToken(user.Payments.CustomerID,
		params.Name, params.Number, params.Month, params.Year, params.CVC)
	if err != nil {
		return "", errors.Wrap(err, "couldn't create card token")
	}
	return token, nil
}

func updateUserCards(user *store.UserMgo) error {
	cards, err := stripe.ListCards(user.Payments.CustomerID)
	if err != nil {
		return errors.Wrap(err, "could not list cards to update user")
	}

	var userCards []*store.UserCard
	for _, c := range cards {
		userCards = append(userCards, &store.UserCard{
			ID:      c.ID,
			Number:  c.Number,
			Year:    c.Year,
			Month:   c.Month,
			Type:    string(c.Type),
			Default: c.Default,
		})
	}
	err = user.SetCards(userCards)
	if err != nil {
		return errors.Wrap(err, "could not update cards on user")
	}

	return nil
}

func deleteUserCard(user *store.UserMgo, cardID string) (*stripe.Card, error) {
	if user.Payments != nil && user.Payments.Cards != nil && len(user.Payments.Cards) > 0 {
		var userCards []*store.UserCard
		var deletedCard *store.UserCard
		for _, card := range user.Payments.Cards {
			if card.ID != cardID {
				userCards = append(userCards, &store.UserCard{
					ID:      card.ID,
					Number:  card.Number,
					Year:    card.Year,
					Month:   card.Month,
					Type:    card.Type,
					Default: card.Default,
				})
			} else {
				deletedCard = card
			}
		}
		if err := user.SetCards(userCards); err != nil {
			return nil, errors.Wrap(err, "could not update cards on user")
		}
		if deletedCard != nil {
			return &stripe.Card{
				ID:      deletedCard.ID,
				Number:  deletedCard.Number,
				Year:    deletedCard.Year,
				Month:   deletedCard.Month,
				Type:    stripe.CardBrand(stripe.GetCardBrand(deletedCard.Type)),
				Default: deletedCard.Default,
			}, nil
		}
	}
	return nil, errors.New("card doesn't exist")
}

func deleteUserBankAccount(user *store.UserMgo) error {
	return user.DeleteBankAccount()
}

// AddCard adds a card to the user account. Handled by Stripe.
func (p *payments) AddCard(user *store.UserMgo, token string) (*stripe.Card, error) {
	if err := p.EnsureCustomer(user); err != nil {
		return nil, errors.Wrapf(err, "couldn't ensure customer to add card (userID:%s)", user.ID.Hex())
	}

	card, err := stripe.NewCard(user.Payments.CustomerID, token)
	if err != nil {
		return nil, errors.Wrap(err, "couldn't add card from token")
	}

	if err := updateUserCards(user); err != nil {
		return nil, err
	}

	return card, nil
}

// DeleteCard removes a card from a user from both Stripe and our database.
func (p *payments) DeleteCard(user *store.UserMgo, id string) (*stripe.Card, error) {
	if user.Payments == nil || user.Payments.CustomerID == "" {
		return nil, errors.New("user does not have a payment account")
	}

	cards, err := stripe.ListCards(user.Payments.CustomerID)
	if err != nil {
		return nil, errors.Wrap(err, "could not list cards to see if there are enough to delete one")
	}

	if len(cards) == 1 {
		return nil, errors.New("can't delete the only card")
	}

	card, err := stripe.DeleteCard(user.Payments.CustomerID, id)
	if err != nil {
		stripeError := err.(*stripeGo.Error)
		if stripeError != nil {
			if stripeError.Code == "resource_missing" && strings.Contains(stripeError.Msg, "No such customer") {
				logger.Get().Errorf("Customer %s does not exist in stripe. Deleting card from database.\n", user.Payments.CustomerID)
				return deleteUserCard(user, id)
			}
		}
		return nil, errors.Wrap(err, "could not delete card")
	}

	if err := updateUserCards(user); err != nil {
		return nil, err
	}

	return card, nil
}

// DeleteCard removes a bank account from a user from both Stripe and our database.
func (p *payments) DeleteBankAccount(user *store.UserMgo, id string) error {
	if user.Payments == nil || user.Payments.CustomerID == "" {
		return errors.New("user does not have a payment account")
	}

	accounts, err := stripe.ListBankAccounts(user.Payments.CustomerID)
	if err != nil {
		return errors.Wrap(err, "could not list bank accounts to see if there are enough to delete one")
	}

	if len(accounts) == 0 {
		return errors.New("no bank accounts in stripe to delete")
	}

	err = stripe.DeleteBankAccount(user.Payments.CustomerID, id)
	if err != nil {
		return errors.Wrap(err, "could not delete bank account")
	}

	if err = deleteUserBankAccount(user); err != nil {
		return err
	}

	return nil
}

// GetCards gets a user's cards from Stripe instead of our database and returns them.
// Can be used for automatic syncing.
func (p *payments) GetCards(user *store.UserMgo) ([]*stripe.Card, error) {
	if user.Payments == nil || user.Payments.CustomerID == "" {
		return nil, errors.New("user does not have a payment account")
	}

	return stripe.ListCards(user.Payments.CustomerID)
}

// NewCustomer creates a new customer and adds it to a user.
func (p *payments) NewCustomer(user *store.UserMgo) (err error) {
	email, err := user.MainEmail()
	if err != nil {
		return errors.Wrap(err, "couldn't get main email")
	}

	id, err := stripe.NewCustomer(user.Name(), email, map[string]string{stripe.MetadataLearntAccountID: user.ID.Hex()})
	if err != nil {
		return err
	}

	return errors.Wrap(user.SetPaymentsCustomer(id), "couldn't set payment customer to user")
}

// CreditForReferral adds credit to the specified user's customer account
func (p *payments) CreditForReferral(user *store.UserMgo, balance float64) error {
	amount := int64(balance * 100)
	if user.Payments != nil && user.Payments.ConnectID != "" {
		description := fmt.Sprintf("Referral credit to %s", user.ID.Hex())
		_, err := stripe.ChargeCorporateCardCompany(user.Payments.ConnectID, amount, description, "", "")
		return err
	}

	if err := p.EnsureCustomer(user); err != nil {
		return errors.Wrapf(err, "couldn't ensure customer to add referral credit (userID:%s)", user.ID.Hex())
	}
	_, err := stripe.CustomerAddToBalance(user.Payments.CustomerID, amount)
	return err
}

func (p *payments) CreditForReferree(user *store.UserMgo, balance float64) error {
	amount := int64(balance * 100)
	if user.Payments != nil && user.Payments.ConnectID != "" {
		description := fmt.Sprintf("Referral credit to %s", user.ID.Hex())
		_, err := stripe.ChargeCorporateCardCompany(user.Payments.ConnectID, amount, description, "", "")
		return err
	}

	if err := p.EnsureCustomer(user); err != nil {
		return errors.Wrapf(err, "couldn't ensure customer to add referral credit (userID:%s)", user.ID.Hex())
	}
	_, err := stripe.CustomerAddToBalance(user.Payments.CustomerID, amount)
	return err
}

// // GetBalance retrieves the balance of the specified user
func (p *payments) GetBalance(user *store.UserMgo) (int64, error) {
	if user.Payments == nil || user.Payments.CustomerID == "" {
		return 0, errors.New("user does not have a payment account")
	}

	if user.IsStudent() {
		return user.Payments.Credits, nil
	}

	return stripe.CustomerGetBalance(user.Payments.CustomerID)
}

//lessonAmounts takes the tutor's rate in DOLLARS and returns the breakdown in CENTS
func lessonAmounts(ratePerHour, durationMinutes float64) (tutorPay, platformFee, studentCost int64) {
	ratePerMinute := (ratePerHour / time.Hour.Minutes()) * 100
	tutorPay = int64(math.Round(ratePerMinute * durationMinutes))
	platformFee = int64(math.Round(platformFeePercentage * float64(tutorPay)))
	if platformFee < 100 {
		platformFee = 100
	}
	studentCost = tutorPay + platformFee
	return tutorPay, platformFee, studentCost
}

func tutorSuffix(tutor *store.UserMgo) (name string) {
	if len(tutor.Name())+6 <= stripe.MAX_DESCRIPTOR_LENGTH {
		name = fmt.Sprintf(" with %s", tutor.Name())
	} else if len(tutor.GetFirstName())+6 <= stripe.MAX_DESCRIPTOR_LENGTH {
		name = fmt.Sprintf(" with %s", tutor.GetFirstName())
	}

	return
}

// ChargeForLesson will create a charge for a lesson that will be confirmed at a future date
func (p *payments) ChargeForLesson(student, tutor *store.UserMgo, duration float64, startDateTime, lessonID string, instant bool, rate float32) (*models.ChargeData, error) {
	if student.Payments == nil {
		return nil, fmt.Errorf("student %v has no payment method for lesson (%s)", student.ID, lessonID)
	}

	tutorPay, platformFee, studentCost := lessonAmounts(float64(rate), duration)
	metadata := map[string]string{"lessonID": lessonID, "student": student.Name(), "tutor": tutor.Name()}
	chargePrefix := prefix
	creditPrefix := "Credit for Lesson with"

	if instant {
		chargePrefix = instantPrefix
		creditPrefix = "Credit for Instant Lesson with"
	}

	var creditsBalance int64
	var toBeDeductedFromCredits int64
	var amountFromLearnt int64
	var description, creditDescription string

	adjustedStudentCost := studentCost
	adjustedFee := platformFee
	adjustedTutorPay := tutorPay

	if student.Payments != nil {
		creditsBalance = student.Payments.Credits
	}

	// Charge to credits first
	if creditsBalance > 0 {
		if creditsBalance >= studentCost {
			// if student credits are greater than the amount to pay,
			// the amount will be deducted from the credits then Learnt will pay the tutor
			toBeDeductedFromCredits = studentCost
			adjustedStudentCost = 0
			adjustedFee = 0
			amountFromLearnt = tutorPay
		} else {
			// if the student's amount to pay is greater than the credits,
			// consume all the credits by deducting from the student's amount to pay
			toBeDeductedFromCredits = creditsBalance
			adjustedStudentCost = studentCost - creditsBalance

			// determine the platform fee and the amount that Learnt will shoulder
			// platform fee will be deducted by the credits first
			if creditsBalance >= platformFee {
				adjustedFee = 0
				amountFromLearnt = tutorPay - adjustedStudentCost
				adjustedTutorPay = adjustedStudentCost
			} else {
				adjustedFee = platformFee - creditsBalance
				amountFromLearnt = 0
			}
		}

		creditsParams := CreditParams{}
		creditsParams.Amount = -1 * toBeDeductedFromCredits
		creditsParams.Reason = "debit"
		creditsParams.Notes = fmt.Sprintf("%s with %s at %s (%s). Charged with %.2f credits", chargePrefix, tutor.Name(), startDateTime, lessonID, float64(toBeDeductedFromCredits)/100)
		if err := GetPayments().AddCredits(student, creditsParams); err != nil {
			return nil, fmt.Errorf("couldn't charge credits for this session: %w", err)
		}
	}

	charge := &models.ChargeData{
		TutorPay:    adjustedTutorPay,
		TutorRate:   rate,
		PlatformFee: platformFee,
		StudentCost: adjustedStudentCost,
	}

	tr := GetTransactions()

	logger.Get().Debugf("charge created for lesson %s: %+v", lessonID, charge)

	if adjustedStudentCost > 0 {
		description = fmt.Sprintf("%s with %s at %s (%s)", chargePrefix, tutor.Name(), startDateTime, lessonID)
		chargeID, err := stripe.Charge(student.Payments.CustomerID, tutor.Payments.ConnectID, adjustedStudentCost, adjustedFee, description, chargePrefix, tutorSuffix(tutor), metadata)
		if err != nil {
			return nil, fmt.Errorf("could not charge for lesson (%s): %w", lessonID, err)
		}

		logger.Get().Debugf("successfully created charge %s", chargeID)

		charge.ChargeID = chargeID

		t := &store.TransactionMgo{
			User:      student.ID,
			Amount:    float64(adjustedStudentCost) / 100,
			Details:   description,
			Reference: chargeID,
		}
		if _, err := tr.New(t); err != nil {
			return nil, fmt.Errorf("couldn't create transaction for student %s on lesson %v: %w", student.Name(), lessonID, err)
		}
		logger.Get().Debugf("created transaction for student %s on lesson %v", student.Name(), lessonID)

		creditDescription = fmt.Sprintf("%s %s at %s (%s)", creditPrefix, student.Name(), startDateTime, lessonID)

		t = &store.TransactionMgo{
			User:    tutor.ID,
			Amount:  float64(adjustedTutorPay) / 100,
			Details: creditDescription,
			State:   store.TransactionSent,
		}
		if _, err = tr.New(t); err != nil {
			return nil, fmt.Errorf("couldn't create transaction for tutor for session: %w", err)
		}
		logger.Get().Debugf("created transaction for tutor for session")
	}

	if amountFromLearnt > 0 {
		description = fmt.Sprintf("Cover customer balance to tutor (lesson:%s)", lessonID)
		chargeID, err := stripe.ChargeCorporateCardCompany(tutor.Payments.ConnectID, amountFromLearnt, description, chargePrefix, tutorSuffix(tutor))
		if err != nil {
			return nil, fmt.Errorf("could not charge amount (%v) for tutor (%s) for lesson (%s) after student credit: %w", amountFromLearnt, tutor.ID.Hex(), lessonID, err)
		}

		charge.ChargeID = chargeID

		creditDescription = fmt.Sprintf("%s %s at %s (%s)", creditPrefix, student.Name(), startDateTime, lessonID)

		t := &store.TransactionMgo{
			User:    tutor.ID,
			Amount:  float64(amountFromLearnt) / 100,
			Details: creditDescription,
			State:   store.TransactionSent,
		}
		if _, err := tr.New(t); err != nil {
			err = errors.Wrap(err, "couldn't create transaction for tutor for session")
		}
	}

	return charge, nil
}

// SetDefaultCreditCard updates a credit card to be the default for charges
func (p *payments) SetDefaultCreditCard(user *store.UserMgo, cardID string) (*stripe.Card, error) {
	if user.Payments == nil || user.Payments.CustomerID == "" {
		return nil, errors.New("user does not have a payment account")
	}

	if err := stripe.CustomerSetDefaultSource(user.Payments.CustomerID, cardID); err != nil {
		return nil, errors.Wrap(err, "could not set default card")
	}

	go func() {
		if err := updateUserCards(user); err != nil {
			logger.Get().Errorf("Could not update user cards", err)
		}
	}()

	return stripe.GetCard(user.Payments.CustomerID, cardID)
}

// SetBankAccount adds or replaces the main bank account of a connect account
func (p *payments) SetBankAccount(user *store.UserMgo, bap *BankAccountParams) (*stripe.BankAccount, error) {
	if user.Payments == nil || user.Payments.ConnectID == "" {
		return nil, errors.New("user does not have a connect account")
	}

	if bap.Token == "" {
		token, err := stripe.NewExternalBankAccountToken(bap.BankAccountName, bap.BankAccountNumber, bap.BankAccountRouting, bap.BankAccountType)
		if err != nil {
			return nil, errors.Wrap(err, "could not create token for bank account")
		}
		bap.Token = token
	}

	ba, err := stripe.ReplaceBankAccount(user.Payments.ConnectID, bap.Token)
	if err != nil {
		return nil, errors.Wrap(err, "could not replace back account in SetBankAccount")
	}

	if err := user.SetBankAccount(store.BankAccount{
		BankAccountID:      ba.ID,
		BankAccountName:    ba.Name,
		BankAccountType:    ba.Type,
		BankAccountNumber:  ba.Number,
		BankAccountRouting: ba.Routing,
	}); err != nil {
		return nil, errors.Wrap(err, "could not set bank account in database")
	}

	return ba, nil
}

func (p *payments) AddCredits(user *store.UserMgo, creditParams CreditParams) error {

	// Add credit to stripe account if user is tutor
	if creditParams.Reason != "debit" && user.IsTutor() {
		if user.Payments != nil && user.Payments.ConnectID != "" {
			if _, err := stripe.ChargeCorporateCardCompany(user.Payments.ConnectID, creditParams.Amount, creditParams.Notes, "", ""); err != nil {
				return errors.Wrap(err, "couldn't charge corporate company card in stripe")
			}
		}
	} else {
		var creditsBalance int64
		if user.Payments != nil {
			creditsBalance = user.Payments.Credits
		}
		totalCredits := creditsBalance + creditParams.Amount

		err := store.GetCollection("users").UpdateId(user.ID, bson.M{
			"$set": bson.M{
				"payments.credits": totalCredits,
			},
		})

		if err != nil {
			return errors.Wrap(err, "couldn't update credits field for this user")
		}
	}

	t := &store.TransactionMgo{
		User:    user.ID,
		Amount:  math.Abs(float64(creditParams.Amount) / 100),
		Details: creditParams.Notes,
		State:   store.TransactionSent,
		Status:  creditParams.Reason,
	}

	if _, err := GetTransactions().New(t); err != nil {
		return errors.Wrap(err, "couldn't create credit transaction for this user")
	}

	return nil
}
