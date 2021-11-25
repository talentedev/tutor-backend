package stripe

import (
	"github.com/cenkalti/backoff"
	"github.com/pkg/errors"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/bankaccount"
	"github.com/stripe/stripe-go/token"
)

// BankAccountStatus alias the stripe type to not need importing of stripe in outside packages
type BankAccountStatus stripe.BankAccountStatus

// BankAccount is a paired down version of the stripe.BankAccount object
type BankAccount struct {
	ID      string `json:"id"`
	Bank    string `json:"bank"`
	Default bool   `json:"default"`
	// https://stripe.com/docs/api/customer_bank_accounts/object?lang=go#customer_bank_account_object-status
	Status BankAccountStatus `json:"status"`

	Name        string `json:"name"`
	Type        string `json:"type"`
	Number      string `json:"number"`
	Routing     string `json:"routing"`
	Fingerprint string
}

func convertBankAccount(ba *stripe.BankAccount) *BankAccount {
	return &BankAccount{
		ID:      ba.ID,
		Bank:    ba.BankName,
		Default: ba.DefaultForCurrency,
		Status:  BankAccountStatus(ba.Status),

		Number:      ba.Last4,
		Name:        ba.AccountHolderName,
		Type:        string(ba.AccountHolderType),
		Routing:     ba.RoutingNumber,
		Fingerprint: ba.Fingerprint,
	}
}

// NewExternalBankAccountToken creates a token to use in the creation of a new bank account
// This could be created in the UI so the parameters are never sent to the API
func NewExternalBankAccountToken(accountHolderName, accountNumber, routingNumber, accountHolderType string) (string, error) {
	params := &stripe.TokenParams{
		BankAccount: &stripe.BankAccountParams{
			AccountHolderName: stringPointerIfNotEmpty(accountHolderName),
			AccountHolderType: stringPointerIfNotEmpty(accountHolderType),
			AccountNumber:     stripe.String(accountNumber),
			RoutingNumber:     stripe.String(routingNumber),
			Country:           stripe.String("US"),
			Currency:          stripe.String("USD"),
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

// NewExternalBankAccount adds an external back account to a stripe account and returns an ID
func NewExternalBankAccount(stripeID, token string) (*BankAccount, error) {

	accountID, customerID := stripeIDSplit(stripeID)
	params := &stripe.BankAccountParams{
		Token:              stripe.String(token),
		DefaultForCurrency: stripe.Bool(true),
		Account:            stringPointerIfNotEmpty(accountID),
		Customer:           stringPointerIfNotEmpty(customerID),
	}

	var ba *stripe.BankAccount
	backoffOperation := func() error {
		var err error
		setIdempotencyKey(params)
		if ba, err = bankaccount.New(params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, wrap(err, "could not add back account")
	}

	switch ba.Status {
	case stripe.BankAccountStatusErrored:
		return nil, errors.New("back account status returned status errored")
		// case stripe.BankAccountStatusVerified:
		// case stripe.BankAccountStatusVerificationFailed:
		// case stripe.BankAccountStatusNew:
		// case stripe.BankAccountStatusValidated:
	}
	return convertBankAccount(ba), nil
}

// GetBankAccount will retrieve one bank account
func GetBankAccount(stripeID, bankAccountID string) (*BankAccount, error) {
	accountID, customerID := stripeIDSplit(stripeID)
	params := &stripe.BankAccountParams{
		Account:  stringPointerIfNotEmpty(accountID),
		Customer: stringPointerIfNotEmpty(customerID),
	}

	var ba *stripe.BankAccount
	backoffOperation := func() error {
		var err error
		if ba, err = bankaccount.Get(bankAccountID, params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, wrap(err, "could not get back account")
	}
	return convertBankAccount(ba), nil
}

// ListBankAccounts returns a list of currently added back accounts
func ListBankAccounts(stripeID string) ([]*BankAccount, error) {
	accountID, customerID := stripeIDSplit(stripeID)
	params := &stripe.BankAccountListParams{
		Account:  stringPointerIfNotEmpty(accountID),
		Customer: stringPointerIfNotEmpty(customerID),
	}

	params.Filters.AddFilter("limit", "", "30")

	var accounts []*BankAccount
	i := bankaccount.List(params)
	for i.Next() {
		accounts = append(accounts, convertBankAccount(i.BankAccount()))
	}
	return accounts, nil
}

// DeleteBankAccount deletes a bank account
func DeleteBankAccount(stripeID, bankAccountID string) error {
	accountID, customerID := stripeIDSplit(stripeID)
	params := &stripe.BankAccountParams{
		Account:  stringPointerIfNotEmpty(accountID),
		Customer: stringPointerIfNotEmpty(customerID),
	}

	backoffOperation := func() error {
		setIdempotencyKey(params)
		if _, err := bankaccount.Del(bankAccountID, params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return wrap(err, "could not delete bank account")
	}
	return nil
}

// ReplaceBankAccount adds a new bank account then deletes all the ones that existed before to leave it as the only and default account
func ReplaceBankAccount(stripeID, token string) (*BankAccount, error) {

	var bas []*BankAccount
	backoffOperation := func() error {
		var err error
		if bas, err = ListBankAccounts(stripeID); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, wrap(err, "could not list accounts")
	}

	var ba *BankAccount
	backoffOperation = func() error {
		var err error
		if ba, err = NewExternalBankAccount(stripeID, token); err != nil {
			return err
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, wrap(err, "could not create new account to replace old ones")
	}

	for _, rba := range bas {
		backoffOperation := func() error {
			if err := DeleteBankAccount(stripeID, rba.ID); err != nil {
				return checkPermanentFailure(err)
			}
			return nil
		}
		if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
			return nil, wrap(err, "could not delete bank account after adding a new one to replace it")
		}
	}

	return ba, nil
}
