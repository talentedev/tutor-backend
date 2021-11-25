package stripe

import (
	"errors"
	"github.com/nyaruka/phonenumbers"
	"time"

	"github.com/cenkalti/backoff"
	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/account"
	"github.com/stripe/stripe-go/person"
)

// MetadataLearntAccountID is a key used in a metadata struct if you want the account to include a id connected to the learnt account
const MetadataLearntAccountID = "LearntID"

// AccountRequirements showss actions that need to be taken to verify the account

// Account is a connect account used by tutors to receive payments
type Account struct {
	ID             string                      `json:"id"`
	ChargesEnabled bool                        `json:"charges_enabled"`
	PayoutsEnabled bool                        `json:"payouts_enabled"`
	Requirements   *stripe.AccountRequirements `json:"requirements"`
	LearntID       string                      `json:"learntId"`
}

func convertAccount(a *stripe.Account) *Account {
	r := &Account{
		ID:             a.ID,
		ChargesEnabled: a.ChargesEnabled,
		PayoutsEnabled: a.PayoutsEnabled,
		Requirements:   a.Requirements,
	}

	if id, ok := a.Metadata[MetadataLearntAccountID]; ok {
		r.LearntID = id
	}
	return r
}

// Address includes information for a street address
type Address struct {
	Address    string
	City       string
	State      string
	PostalCode string
	Country    string
}

// AccountParams are all the things needed to create a connect account
// https://stripe.com/docs/connect/required-verification-information
type AccountParams struct {
	TermsAccepted     bool
	TermsAcceptanceIP string
	Email             string
	FirstName         string
	LastName          string
	Telephone         string
	Birthday          time.Time
	SSNLast4          string
	Address           Address
	MetaData          map[string]string

	// Only For Business Accounts
	CompanyEIN  string
	CompanyName string
}

func (ap *AccountParams) validate() error {
	if !ap.TermsAccepted {
		return errors.New("user has not accepted the payout terms")
	}

	if ap.Email == "" {
		return errors.New("account params was missing an email")
	}

	phone, err := phonenumbers.Parse(ap.Telephone, "US")
	if err != nil || !phonenumbers.IsValidNumber(phone) {
		return errors.New("telephone number is invalid")
	}

	if v, ok := states[ap.Address.State]; ok {
		ap.Address.State = v
	}

	return nil
}

// NewAccount will create an account that can receive payments it returns an account number
func NewAccount(accountParams *AccountParams) (*Account, error) {
	if err := accountParams.validate(); err != nil {
		return nil, err
	}

	params := &stripe.AccountParams{
		Type:  stripe.String(string(stripe.AccountTypeCustom)),
		Email: stripe.String(accountParams.Email),
		RequestedCapabilities: []*string{
			stripe.String(string(stripe.AccountCapabilityTransfers)),
			stripe.String(string(stripe.AccountCapabilityCardPayments)),
		},
		BusinessProfile: &stripe.AccountBusinessProfileParams{
			MCC:                stripe.String(merchantCategory),
			ProductDescription: stripe.String("Tutor"),
			// TODO once stripe accepts a url, make this point to the tutors page
			// URL: stripe.String("https://example.com"),
		},
		TOSAcceptance: &stripe.AccountTOSAcceptanceParams{
			//TODO maybe change the user.Preferences to include a date of when they accepted the terms
			Date: stripe.Int64(time.Now().Unix()),
			IP:   stripe.String(accountParams.TermsAcceptanceIP),
		},
		Settings: &stripe.AccountSettingsParams{
			Payouts: &stripe.AccountSettingsPayoutsParams{
				// TODO verify these details
				StatementDescriptor: stripe.String("Learnt"),
				Schedule: &stripe.PayoutScheduleParams{
					DelayDays: stripe.Int64(2),
					Interval:  stripe.String("daily"),
				},
			},
		},
	}

	if accountParams.CompanyEIN != "" {
		params.BusinessType = stripe.String(string(stripe.AccountBusinessTypeCompany))
		params.Company = &stripe.AccountCompanyParams{
			Name:  stripe.String(accountParams.CompanyName),
			TaxID: stripe.String(accountParams.CompanyEIN),
			Phone: stripe.String(accountParams.Telephone),
			Address: &stripe.AccountAddressParams{
				Line1:      stripe.String(accountParams.Address.Address),
				City:       stripe.String(accountParams.Address.City),
				State:      stripe.String(accountParams.Address.State),
				PostalCode: stripe.String(accountParams.Address.PostalCode),
				Country:    stripe.String(accountParams.Address.Country),
			},
		}
	} else { // Individual
		params.BusinessType = stripe.String(string(stripe.AccountBusinessTypeIndividual))
		params.Individual = &stripe.PersonParams{
			FirstName: stripe.String(accountParams.FirstName),
			LastName:  stripe.String(accountParams.LastName),
			Email:     stripe.String(accountParams.Email),
			Phone:     stripe.String(accountParams.Telephone),
			DOB: &stripe.DOBParams{
				Day:   stripe.Int64(int64(accountParams.Birthday.Day())),
				Month: stripe.Int64(int64(accountParams.Birthday.Month())),
				Year:  stripe.Int64(int64(accountParams.Birthday.Year())),
			},
			SSNLast4: stripe.String(accountParams.SSNLast4),
			Address: &stripe.AccountAddressParams{
				Line1:      stripe.String(accountParams.Address.Address),
				City:       stripe.String(accountParams.Address.City),
				State:      stripe.String(accountParams.Address.State),
				PostalCode: stripe.String(accountParams.Address.PostalCode),
				Country:    stripe.String(accountParams.Address.Country),
			},
		}
	}

	for k, v := range accountParams.MetaData {
		params.AddMetadata(k, v)
	}

	var acct *stripe.Account
	backoffOperation := func() error {
		var err error
		setIdempotencyKey(params)
		if acct, err = account.New(params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, wrap(err, "could not create new stripe account")
	}

	if accountParams.CompanyEIN == "" {
		return convertAccount(acct), nil
	}

	personParams := &stripe.PersonParams{
		Account:   stripe.String(acct.ID),
		FirstName: stripe.String(accountParams.FirstName),
		LastName:  stripe.String(accountParams.LastName),
		Email:     stripe.String(accountParams.Email),
		Phone:     stripe.String(accountParams.Telephone),
		SSNLast4:  stripe.String(accountParams.SSNLast4),
		DOB: &stripe.DOBParams{
			Day:   stripe.Int64(int64(accountParams.Birthday.Day())),
			Month: stripe.Int64(int64(accountParams.Birthday.Month())),
			Year:  stripe.Int64(int64(accountParams.Birthday.Year())),
		},
		Address: &stripe.AccountAddressParams{
			Line1:      stripe.String(accountParams.Address.Address),
			City:       stripe.String(accountParams.Address.City),
			State:      stripe.String(accountParams.Address.State),
			PostalCode: stripe.String(accountParams.Address.PostalCode),
			Country:    stripe.String(accountParams.Address.Country),
		},
		Relationship: &stripe.RelationshipParams{
			// Field removed due stripe version updated from v60.2.0 to v70.15.0
			// AccountOpener:    stripe.Bool(true),
			Owner:            stripe.Bool(true),
			Title:            stripe.String("Owner"),
			PercentOwnership: stripe.Float64(100),
		},
	}

	backoffOperation = func() error {
		if _, err := person.New(personParams); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, wrap(err, "could not add AccountOpener to the business")
	}

	return GetAccount(acct.ID)

}

// GetAccount gets an account from stripe
func GetAccount(accountID string) (*Account, error) {
	var acct *stripe.Account
	backoffOperation := func() error {
		var err error
		if acct, err = account.GetByID(accountID, nil); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return nil, wrap(err, "could not get account")
	}

	return convertAccount(acct), nil
}

// DeleteAccount deletes a stripe account that receives payments
func DeleteAccount(accountID string) error {
	params := &stripe.AccountParams{}
	backoffOperation := func() error {
		setIdempotencyKey(params)
		if _, err := account.Del(accountID, params); err != nil {
			return checkPermanentFailure(err)
		}
		return nil
	}
	if err := backoff.Retry(backoffOperation, exponentialBackOff()); err != nil {
		return wrap(err, "could not delete stripe account")
	}
	return nil
}

var states = map[string]string{
	"Alabama":              "AL",
	"Alaska":               "AK",
	"Arizona":              "AZ",
	"Arkansas":             "AR",
	"California":           "CA",
	"Colorado":             "CO",
	"Connecticut":          "CT",
	"Delaware":             "DE",
	"District of Columbia": "DC",
	"Florida":              "FL",
	"Georgia":              "GA",
	"Hawaii":               "HI",
	"Idaho":                "ID",
	"Illinois":             "IL",
	"Indiana":              "IN",
	"Iowa":                 "IA",
	"Kansas":               "KS",
	"Kentucky":             "KY",
	"Louisiana":            "LA",
	"Maine":                "ME",
	"Maryland":             "MD",
	"Massachusetts":        "MA",
	"Michigan":             "MI",
	"Minnesota":            "MN",
	"Mississippi":          "MS",
	"Missouri":             "MO",
	"Montana":              "MT",
	"Nebraska":             "NE",
	"Nevada":               "NV",
	"New Hampshire":        "NH",
	"New Jersey":           "NJ",
	"New Mexico":           "NM",
	"New York":             "NY",
	"North Carolina":       "NC",
	"North Dakota":         "ND",
	"Ohio":                 "OH",
	"Oklahoma":             "OK",
	"Oregon":               "OR",
	"Pennsylvania":         "PA",
	"Rhode Island":         "RI",
	"South Carolina":       "SC",
	"South Dakota":         "SD",
	"Tennessee":            "TN",
	"Texas":                "TX",
	"Utah":                 "UT",
	"Vermont":              "VT",
	"Virginia":             "VA",
	"Washington":           "WA",
	"West Virginia":        "WV",
	"Wisconsin":            "WI",
	"Wyoming":              "WY",
	"Puerto Rico":          "PR",
}
