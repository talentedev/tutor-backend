package stripe

import (
	"testing"

	"github.com/stripe/stripe-go/charge"
)

func TestCharge(t *testing.T) {
	custID := happyCustomer(t, "Charge Customer")
	defer deleteCustomer(t, custID)
	if _, err := NewCard(custID, testCardToken); err != nil {
		t.Fatal("Could not add new card to customer", err)
	}

	description := "Test charge"
	// This test uses a verifiedAccountID because it takes longer than 30 seconds to verify a new account
	chargeID, err := Charge(custID, verifiedAccountID, 1000, 100, description, "", "", nil)
	if err != nil {
		t.Fatal("Could not test charge for lesson", err)
	}

	ch, err := charge.Get(chargeID, nil)
	if err != nil {
		t.Fatal("could not get charge", err)
	}

	if ch.Status != "succeeded" {
		t.Fatal("charge status should be succeeded", ch.Status)
	}
}

func TestStatementDescriptorField(t *testing.T) {
	custID := happyCustomer(t, "Charge Customer")
	defer deleteCustomer(t, custID)
	if _, err := NewCard(custID, testCardToken); err != nil {
		t.Fatal("Could not add new card to customer", err)
	}

	description := "Test charge"
	prefix := "Test Prefix"
	suffix := "Test Suffix"

	// This test uses a verifiedAccountID because it takes longer than 30 seconds to verify a new account
	chargeID, err := Charge(custID, verifiedAccountID, 1000, 100, description, prefix, suffix, nil)
	if err != nil {
		t.Fatal("Could not test charge for lesson", err)
	}

	ch, err := charge.Get(chargeID, nil)
	if err != nil {
		t.Fatal("could not get charge", err)
	}

	if ch.Status != "succeeded" {
		t.Fatal("charge status should be succeeded", ch.Status)
	}

	if ch.StatementDescriptor != prefix {
		t.Fatal("statement descriptor should be the same as description")
	}

	if ch.StatementDescriptorSuffix != suffix {
		t.Fatal("statement descriptor suffix should be the same as description")
	}
}

func TestStatementDescriptorPrefixFieldFail(t *testing.T) {
	custID := happyCustomer(t, "Charge Customer")
	defer deleteCustomer(t, custID)
	if _, err := NewCard(custID, testCardToken); err != nil {
		t.Fatal("Could not add new card to customer", err)
	}

	description := "Test charge"
	suffix := "Test Suffix"
	prefix := "Test description longer than 22 characters."

	// This test uses a verifiedAccountID because it takes longer than 30 seconds to verify a new account
	_, err := Charge(custID, verifiedAccountID, 1000, 100, description, prefix, suffix, nil)
	if err == nil {
		t.Errorf("should error on prefix longer than %d", MAX_DESCRIPTOR_LENGTH)
	}
}

func TestStatementDescriptorSuffixFieldFail(t *testing.T) {
	custID := happyCustomer(t, "Charge Customer")
	defer deleteCustomer(t, custID)
	if _, err := NewCard(custID, testCardToken); err != nil {
		t.Fatal("Could not add new card to customer", err)
	}

	description := "Test charge"
	suffix := "Test description longer than 22 characters."
	prefix := "Test Prefix"

	// This test uses a verifiedAccountID because it takes longer than 30 seconds to verify a new account
	_, err := Charge(custID, verifiedAccountID, 1000, 100, description, prefix, suffix, nil)
	if err == nil {
		t.Errorf("should error on prefix longer than %d", MAX_DESCRIPTOR_LENGTH)
	}
}

func TestChargeAuthorizeCaputure(t *testing.T) {
	custID := happyCustomer(t, "Charge Customer")
	defer deleteCustomer(t, custID)
	if _, err := NewCard(custID, testCardToken); err != nil {
		t.Fatal("Could not add new card to customer", err)
	}

	description := "Test charge authorization"
	// This test uses a verifiedAccountID because it takes longer than 30 seconds to verify a new account
	chargeID, err := AuthorizeCharge(custID, verifiedAccountID, 1000, 100, description, "", "", nil)
	if err != nil {
		t.Fatal("Could not test charge for lesson", err)
	}

	ch, err := charge.Get(chargeID, nil)
	if err != nil {
		t.Fatal("could not get charge", err)
	}

	if ch.Captured {
		t.Fatal("charge should not have been captured yet")
	}

	chargeID2, err := CaptureCharge(chargeID)
	if err != nil {
		t.Fatal("could not capture charge", err)
	}

	if chargeID != chargeID2 {
		t.Fatal("capturing the charge returned a different id")
	}

	ch, err = charge.Get(chargeID, nil)
	if err != nil {
		t.Fatal("could not get charge", err)
	}

	if !ch.Captured {
		t.Fatal("charge should have been captured now")
	}
}

func TestUpdateAndCaptureCharge(t *testing.T) {
	custID := happyCustomer(t, "Charge Customer")
	defer deleteCustomer(t, custID)
	if _, err := NewCard(custID, testCardToken); err != nil {
		t.Fatal("Could not add new card to customer", err)
	}

	chargeID, err := AuthorizeCharge(custID, verifiedAccountID, 1000, 100, "Test cancel", "", "", nil)
	if err != nil {
		t.Fatal("Could not test charge for lesson", err)
	}

	reducedAmount := int64(800)
	reducedFee := int64(90)
	if _, err := UpdateAndCaptureCharge(chargeID, reducedAmount, reducedFee, nil); err != nil {
		t.Fatal("Could not update and capture charge", err)
	}

	ch, err := charge.Get(chargeID, nil)
	if err != nil {
		t.Fatal("Could not get charge", err)
	}

	if !ch.Paid {
		t.Fatal("Charge should be paid", err)
	}

	if ch.Amount-ch.AmountRefunded != reducedAmount {
		t.Fatal("Amount and refund did not match the reduced amount", ch.Amount, ch.AmountRefunded, reducedAmount)
	}

	if ch.ApplicationFeeAmount != reducedFee {
		t.Fatal("Fee and refund did not match the reduced amount", ch.ApplicationFeeAmount, reducedFee)
	}
}

func TestCancelCharge(t *testing.T) {
	custID := happyCustomer(t, "Charge Customer")
	defer deleteCustomer(t, custID)
	if _, err := NewCard(custID, testCardToken); err != nil {
		t.Fatal("Could not add new card to customer", err)
	}

	chargeID, err := AuthorizeCharge(custID, verifiedAccountID, 1000, 100, "Test cancel", "", "", nil)
	if err != nil {
		t.Fatal("Could not authorize charge for lesson", err)
	}

	if _, err := CancelCharge(chargeID); err != nil {
		t.Fatal("Could not cancel charge for lesson")
	}
}

func TestChargeCorporateCardCompany(t *testing.T) {
	description := "test coporate card charge"
	if _, err := ChargeCorporateCardCompany(verifiedAccountID, 100, description, "", ""); err != nil {
		t.Fatal("Could not charge corporate card", err)
	}
}
