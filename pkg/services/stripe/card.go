package stripe

import (
	"errors"
	"fmt"

	"github.com/cenkalti/backoff"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/card"
	"github.com/stripe/stripe-go/token"
	"gitlab.com/learnt/api/pkg/logger"
)

// CardBrand is the company that the card belongs to
type CardBrand stripe.CardBrand

// Card is a paired down version of a stripe card
type Card struct {
	ID      string    `json:"id"`
	Number  string    `json:"number"`
	Year    uint16    `json:"year"`
	Month   uint8     `json:"month"`
	Type    CardBrand `json:"type"`
	Default bool      `json:"default"`
}

func convertCard(c *stripe.Card) *Card {
	return &Card{
		ID:      c.ID,
		Type:    CardBrand(c.Brand),
		Number:  c.Last4,
		Default: c.DefaultForCurrency,
		Month:   c.ExpMonth,
		Year:    c.ExpYear,
	}
}

func GetCardBrand(cardType string) stripe.CardBrand {
	switch cardType {
	case string(stripe.CardBrandAmex):
		return stripe.CardBrandAmex
	case string(stripe.CardBrandDiscover):
		return stripe.CardBrandDiscover
	case string(stripe.CardBrandDinersClub):
		return stripe.CardBrandDinersClub
	case string(stripe.CardBrandJCB):
		return stripe.CardBrandJCB
	case string(stripe.CardBrandMasterCard):
		return stripe.CardBrandMasterCard
	case string(stripe.CardBrandUnknown):
		return stripe.CardBrandUnknown
	case string(stripe.CardBrandUnionPay):
		return stripe.CardBrandUnionPay
	case string(stripe.CardBrandVisa):
		return stripe.CardBrandVisa
	default:
		return stripe.CardBrandUnknown
	}
}

// NewCardToken creates a token to use in a request to create a new card
// This could be created in the UI so the parameters are never sent to the API
func NewCardToken(stripeID, name, number, expMonth, expYear, cvc string) (string, error) {
	exists, err := cardExists(stripeID, number, expMonth, expYear)
	if err != nil {
		return "", wrap(err, "could not check if card exists before creating token")
	}
	if exists {
		return "", errors.New("card already exists")
	}

	params := &stripe.TokenParams{
		Card: &stripe.CardParams{
			Name:     stringPointerIfNotEmpty(name),
			Number:   stripe.String(number),
			ExpMonth: stripe.String(expMonth),
			ExpYear:  stripe.String(expYear),
			CVC:      stripe.String(cvc),
		},
	}

	var t *stripe.Token
	backoffOperation := func() error {
		var err error
		setIdempotencyKey(params)
		if t, err = token.New(params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return "", wrap(err, "could not create card token")
	}
	return t.ID, nil
}

func cardExists(stripeID, number, expMonth, expYear string) (bool, error) {
	cards, err := ListCards(stripeID)
	if err != nil {
		return false, wrap(err, "could not list cards to see if one exists")
	}

	last4 := number[len(number)-4:]
	for _, card := range cards {
		year := fmt.Sprintf("%v", card.Year)
		month := fmt.Sprintf("%v", card.Month)
		if card.Number == last4 && month == expMonth && year == expYear {
			return true, nil
		}
	}

	return false, nil
}

// NewCard adds a card an account or customer
func NewCard(stripeID, token string) (*Card, error) {
	accountID, customerID := stripeIDSplit(stripeID)
	params := &stripe.CardParams{
		Token:    stripe.String(token),
		Account:  stringPointerIfNotEmpty(accountID),
		Customer: stringPointerIfNotEmpty(customerID),
	}

	var c *stripe.Card
	backoffOperation := func() error {
		var err error
		setIdempotencyKey(params)
		if c, err = card.New(params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, wrap(err, "could not add new card")
	}
	logger.Get().Infof("CardID (%s) added to stripe account (%s)", c.ID, stripeID)
	return convertCard(c), nil
}

// DeleteCard removes a card
func DeleteCard(stripeID, cardID string) (*Card, error) {
	accountID, customerID := stripeIDSplit(stripeID)
	params := &stripe.CardParams{
		Account:  stringPointerIfNotEmpty(accountID),
		Customer: stringPointerIfNotEmpty(customerID),
	}

	var c *stripe.Card
	backoffOperation := func() error {
		var err error
		setIdempotencyKey(params)
		if c, err = card.Del(cardID, params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, err
	}
	return convertCard(c), nil
}

// ListCards lists all cards associated with an account or customer
func ListCards(stripeID string) ([]*Card, error) {
	accountID, customerID := stripeIDSplit(stripeID)
	params := &stripe.CardListParams{
		Account:  stringPointerIfNotEmpty(accountID),
		Customer: stringPointerIfNotEmpty(customerID),
	}
	params.Filters.AddFilter("limit", "", "30")

	var cards []*Card
	i := card.List(params)
	for i.Next() {
		cards = append(cards, convertCard(i.Card()))
	}
	return cards, nil
}

// GetCard will get a card by an ID
func GetCard(stripeID, cardID string) (*Card, error) {
	accountID, customerID := stripeIDSplit(stripeID)
	params := &stripe.CardParams{
		Account:  stringPointerIfNotEmpty(accountID),
		Customer: stringPointerIfNotEmpty(customerID),
	}

	var c *stripe.Card
	backoffOperation := func() error {
		var err error
		if c, err = card.Get(cardID, params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, wrap(err, "could not get card by id")
	}
	return convertCard(c), nil
}
