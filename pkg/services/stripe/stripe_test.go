package stripe

import (
	"testing"
)

// https://stripe.com/docs/testing#cards
// https://stripe.com/docs/connect/testing

var testCardToken = "tok_visa"
var testDebitToken = "tok_visa_debit"
var testCardNumber = "4242424242424242"
var testTaxID = "000000000"

var verifiedAccountID = "acct_1EDPJ8GLmiCcblkW"

func TestMain(m *testing.M) {
	// TODO: Refactor
	//app.GetConfig().Set("service.stripe.secret", "sk_test_Z61VlRQnhV04OKbddmhQ9TB5")
	//Init()
	//os.Exit(m.Run())
}
