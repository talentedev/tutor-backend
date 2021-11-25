package stripe

import (
	"net/http"
	"strings"
	"time"

	"github.com/cenkalti/backoff"
	"github.com/google/uuid"
	"github.com/pkg/errors"
	stripe "github.com/stripe/stripe-go"
	"gitlab.com/learnt/api/config"
)

// https://stripe.com/docs/issuing/authorizations/categories
var merchantCategory = "8299" //educational_services

// Init will init the stripe client
func Init() {
	stripe.Key = config.GetConfig().GetString("service.stripe.secret")
	stripe.DefaultLeveledLogger = &stripe.LeveledLogger{
		Level: stripe.LevelWarn,
	}
}

func wrap(err error, msg string) error {
	if se, ok := err.(*stripe.Error); ok {
		err = errors.New(se.Msg)
	}
	return errors.Wrap(err, msg)
}

func stringPointerIfNotEmpty(s string) *string {
	if s != "" {
		return &s
	}
	return nil
}

func stripeIDSplit(id string) (accountID, customerID string) {
	if strings.HasPrefix(id, "cus") {
		customerID = id
	} else {
		accountID = id
	}
	return
}

type param interface {
	SetIdempotencyKey(string)
}

func setIdempotencyKey(p param) {
	p.SetIdempotencyKey(uuid.New().String())
}

func exponentialBackOff() *backoff.ExponentialBackOff {
	eb := backoff.NewExponentialBackOff()
	eb.MaxElapsedTime = time.Minute
	eb.InitialInterval = time.Second
	return eb
}

func checkPermanentFailure(err error) error {
	if se, ok := err.(*stripe.Error); ok {
		switch se.HTTPStatusCode {
		case http.StatusBadRequest, http.StatusNotFound, http.StatusPaymentRequired:
			return backoff.Permanent(err)
		}
	}
	return err
}
