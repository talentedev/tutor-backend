package stripe

import (
	"github.com/cenkalti/backoff"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/customer"
)

// NewCustomer will create a customer account that can pay for things
func NewCustomer(name, email string, metadata map[string]string) (string, error) {
	params := &stripe.CustomerParams{
		Description: stripe.String(name),
		Email:       stripe.String(email),
	}

	for k, v := range metadata {
		params.AddMetadata(k, v)
	}

	var cus *stripe.Customer
	backoffOperation := func() error {
		var err error
		setIdempotencyKey(params)
		if cus, err = customer.New(params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return "", wrap(err, "could not create customer")
	}
	return cus.ID, nil
}

// DeleteCustomer deletes a customer, someone who pays for things
func DeleteCustomer(customerID string) error {
	backoffOperation := func() error {
		if _, err := customer.Del(customerID, nil); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return wrap(err, "could not delete customer")
	}
	return nil
}

// CustomerGetBalance gets the current balance of a customer
func CustomerGetBalance(customerID string) (int64, error) {
	var c *stripe.Customer
	backoffOperation := func() error {
		var err error
		if c, err = customer.Get(customerID, nil); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return 0, wrap(err, "could not get customer to get current balance")
	}
	return c.Balance, nil
}

// CustomerAddToBalance adds the amoutn to the current customer balance
func CustomerAddToBalance(customerID string, amount int64) (int64, error) {
	var balance int64
	backoffOperation := func() error {
		var err error
		if balance, err = CustomerGetBalance(customerID); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return 0, wrap(err, "could not get customer balance")
	}
	sum := amount + balance
	params := &stripe.CustomerParams{
		Balance: stripe.Int64(sum),
	}

	backoffOperation = func() error {
		if _, err := customer.Update(customerID, params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return 0, wrap(err, "could not update balance on customer")
	}
	return sum, nil
}

// CustomerSetDefaultSource set the default source for payments. This can be a card or bank account
func CustomerSetDefaultSource(customerID string, sourceID string) error {
	params := &stripe.CustomerParams{
		DefaultSource: stripe.String(sourceID),
	}
	backoffOperation := func() error {
		if _, err := customer.Update(customerID, params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return wrap(err, "could not update the customer default payment source")
	}
	return nil
}
