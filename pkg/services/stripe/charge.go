package stripe

import (
	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/charge"
	"github.com/stripe/stripe-go/refund"
	"gitlab.com/learnt/api/config"
)

const (
	MAX_DESCRIPTOR_LENGTH = 22
)

/**
 * TO-DO: Charge API is unmaintained now as Stripe has new PaymentIntent for unifying all their products and payment services.
 * While Charge isn't deprecated, all new features of Stripe are added now onto PaymentIntent.
 * see https://stripe.com/docs/payments/payment-intents/migration/charges
 */
func newCharge(params *stripe.ChargeParams) (*stripe.Charge, error) {

	var ch *stripe.Charge
	backoffOperation := func() error {
		var err error
		setIdempotencyKey(params)
		if ch, err = charge.New(params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, wrap(err, "could not create new charge")
	}
	return ch, nil
}

// Charge will charge customer and delieve the amount minus the fee to the payee
func Charge(customer, payeeAccount string, amount, fee int64, description, prefix, suffix string, metadata map[string]string) (string, error) {
	if len(prefix) > MAX_DESCRIPTOR_LENGTH {
		return "", errors.New("statement descriptor prefix longer than required length") // we either use this or not
	}

	if len(suffix) > MAX_DESCRIPTOR_LENGTH {
		return "", errors.New("statement descriptor suffix longer than required length") // we either use this or not
	}

	params := &stripe.ChargeParams{
		Amount:                    stripe.Int64(amount),
		ApplicationFeeAmount:      stripe.Int64(fee),
		Currency:                  stripe.String(string(stripe.CurrencyUSD)),
		Customer:                  stripe.String(customer),
		Description:               stripe.String(description),
		StatementDescriptor:       stringPointerIfNotEmpty(prefix),
		StatementDescriptorSuffix: stringPointerIfNotEmpty(suffix),
		TransferData: &stripe.ChargeTransferDataParams{
			Destination: stripe.String(payeeAccount),
		},
	}

	for k, v := range metadata {
		params.AddMetadata(k, v)
	}

	ch, err := newCharge(params)
	if err != nil {
		return "", wrap(err, "could not authorize charge")
	}

	return ch.ID, nil
}

// AuthorizeCharge will confirm with that they payment can be made and hold the charge for up to 7 days to be confrimed.
// This will pay an account while taking a fee. Amount and fee are in cents
//  statementDescriptor will customize the text that will show up on the customer's bank statement
func AuthorizeCharge(customer, payeeAccount string, amount, fee int64, description, prefix, suffix string, metadata map[string]string) (string, error) {
	if len(prefix) > MAX_DESCRIPTOR_LENGTH {
		return "", errors.New("statement descriptor prefix longer than required length") // we either use this or not
	}

	if len(suffix) > MAX_DESCRIPTOR_LENGTH {
		return "", errors.New("statement descriptor suffix longer than required length") // we either use this or not
	}
	params := &stripe.ChargeParams{
		Amount:                    stripe.Int64(amount),
		ApplicationFeeAmount:      stripe.Int64(fee),
		Currency:                  stripe.String(string(stripe.CurrencyUSD)),
		Customer:                  stripe.String(customer),
		Description:               stripe.String(description),
		StatementDescriptor:       stringPointerIfNotEmpty(prefix),
		StatementDescriptorSuffix: stringPointerIfNotEmpty(suffix),
		Capture:                   stripe.Bool(false),
		TransferData: &stripe.ChargeTransferDataParams{
			Destination: stripe.String(payeeAccount),
		},
	}

	for k, v := range metadata {
		params.AddMetadata(k, v)
	}

	ch, err := newCharge(params)
	if err != nil {
		return "", wrap(err, "could not authorize charge")
	}
	return ch.ID, nil
}

// CaptureCharge will make a charged that has been authorized to be charged
func CaptureCharge(chargeID string) (string, error) {
	var ch *stripe.Charge
	backoffOperation := func() error {
		var err error
		if ch, err = charge.Capture(chargeID, nil); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return "", wrap(err, "could not capture charge")
	}
	return ch.ID, nil
}

// UpdateAndCaptureCharge update the charge before capturing it
func UpdateAndCaptureCharge(chargeID string, amount, fee int64, metadata map[string]string) (string, error) {
	params := &stripe.CaptureParams{
		Amount:               stripe.Int64(amount),
		ApplicationFeeAmount: stripe.Int64(fee),
	}
	for k, v := range metadata {
		params.AddMetadata(k, v)
	}

	var ch *stripe.Charge
	backoffOperation := func() error {
		var err error
		if ch, err = charge.Capture(chargeID, params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return "", wrap(err, "could not update the charge")
	}
	return ch.ID, nil
}

// CancelCharge will call the refund api to cancel a charge. This should be used instead of capturing
func CancelCharge(chargeID string) (string, error) {
	params := &stripe.RefundParams{
		Charge: stripe.String(chargeID),
	}

	r, err := refund.New(params)
	if err != nil {
		return "", wrap(err, "could not cancel(refund) uncaptured charge")
	}

	return r.ID, nil
}

// ChargeCorporateCardCompany charges a corporate card
func ChargeCorporateCardCompany(payeeAccount string, amount int64, description, prefix, suffix string) (string, error) {
	if amount <= 0 {
		return "", errors.New("attempting to charge the corporate card a zero or negative amount")
	}

	customer := config.GetConfig().GetString("service.stripe.coporate_charge_customer")
	if customer == "" {
		return "", errors.Errorf("no coporate charge customer account set to send %v to %s", amount, payeeAccount)
	}

	if len(prefix) > MAX_DESCRIPTOR_LENGTH {
		return "", errors.New("statement descriptor prefix longer than required length")
	}

	if len(suffix) > MAX_DESCRIPTOR_LENGTH {
		return "", errors.New("statement descriptor suffix longer than required length")
	}

	params := &stripe.ChargeParams{
		Amount:                    stripe.Int64(amount),
		Currency:                  stripe.String(string(stripe.CurrencyUSD)),
		Customer:                  stripe.String(customer),
		Description:               stripe.String(description),
		StatementDescriptor:       stringPointerIfNotEmpty(prefix),
		StatementDescriptorSuffix: stringPointerIfNotEmpty(suffix),
		TransferData: &stripe.ChargeTransferDataParams{
			Destination: stripe.String(payeeAccount),
		},
	}

	ch, err := newCharge(params)
	if err != nil {
		return "", wrap(err, "could not charge coporate card")
	}

	return ch.ID, nil
}
