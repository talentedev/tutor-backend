package stripe

import (
	"testing"
	"time"

	stripe "github.com/stripe/stripe-go"
	"github.com/stripe/stripe-go/card"
	"gopkg.in/mgo.v2/bson"
)

func TestForceDelete(t *testing.T) {
	// This is just used to cleanup if you manually test
	toDelete := []string{}
	for _, v := range toDelete {
		deleteAccount(t, v)
	}
}

func happyAccountParams(firstName string) *AccountParams {
	ap := AccountParams{
		TermsAccepted:     true,
		TermsAcceptanceIP: "127.0.0.1",
		Email:             "unitTest@example.com",
		FirstName:         "Created",
		LastName:          time.Now().Format(time.RFC3339),
		Telephone:         "5551234444",
		Birthday:          time.Date(2000, time.January, 1, 0, 0, 0, 0, time.UTC),
		SSNLast4:          "0000",
		Address: Address{
			Address:    "87 Lafayette Street",
			City:       "New York City",
			State:      "NY",
			PostalCode: "10013",
			Country:    "US",
		},
		MetaData: map[string]string{"testUserID": bson.NewObjectId().Hex()},
	}

	if firstName != "" {
		ap.FirstName = firstName
	}

	return &ap
}

func deleteAccount(t *testing.T, id string) {
	if err := DeleteAccount(id); err != nil {
		t.Error("Could not delete account", err)
	}
}

func newAccount(t *testing.T, firstName string) string {
	ap := happyAccountParams(firstName)
	account, err := NewAccount(ap)
	if err != nil {
		t.Fatal("Could not create account:", err)
	}
	return account.ID
}

func addDebitCardToAccount(t *testing.T, acctID string) {
	params := &stripe.CardParams{
		Account: stripe.String(acctID),
		Token:   stripe.String(testDebitToken),
	}
	if _, err := card.New(params); err != nil {
		t.Fatal("Could not add debit card to account")
	}
}

func TestNewAccount(t *testing.T) {
	tests := []struct {
		name string
		ap   *AccountParams
	}{
		{name: "Individual", ap: happyAccountParams("Individual")},
		{name: "Company",
			ap: func() *AccountParams {
				ap := happyAccountParams("Company")
				ap.CompanyEIN = testTaxID
				ap.CompanyName = "Company Name"
				return ap
			}(),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			account, err := NewAccount(test.ap)
			if err != nil {
				t.Fatal("Could not create account:", err)
			}
			defer deleteAccount(t, account.ID)
			addDebitCardToAccount(t, account.ID)

			timeout := time.After(20 * time.Second)
			done := make(chan bool)
			go func() {

				for {
					// Give stripe enough time to verify the address so it doesn't come up as a currently due requirement
					time.Sleep(3 * time.Second)

					account, err = GetAccount(account.ID)
					if err != nil {
						t.Error("Could not look up the new account by ID")
						break
					}

					if len(account.Requirements.CurrentlyDue) != 0 {
						t.Log("Waitng because there are currently due requirements", account.Requirements.CurrentlyDue)
						continue
					}
					break
				}
				done <- true

			}()
			select {
			case <-timeout:
				t.Fatal("Test didn't finish in time")
			case <-done:
			}
		})
	}
}

func TestChangeStateToAbreviation(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{in: "NY", out: "NY"},
		{in: "New York", out: "NY"},
		{in: "Cat", out: "Cat"},
	}

	for _, test := range tests {
		t.Run(test.in, func(t *testing.T) {
			ap := AccountParams{TermsAccepted: true, Email: "a@b.c"}
			ap.Address.State = test.in
			ap.validate()

			if ap.Address.State != test.out {
				t.Fatal("incorrect ouput:", ap.Address.State)
			}
		})
	}

}
