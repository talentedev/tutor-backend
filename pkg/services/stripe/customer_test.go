package stripe

import (
	"testing"

	"gopkg.in/mgo.v2/bson"
)

func deleteCustomer(t *testing.T, id string) {
	if err := DeleteCustomer(id); err != nil {
		t.Error("Could not delete customer", err)
	}
}

func happyCustomer(t *testing.T, name string) string {
	id, err := NewCustomer(name, "customer@example.com", map[string]string{"testCustomerID": bson.NewObjectId().Hex()})
	if err != nil {
		t.Fatal("Could not create customer", err)
	}
	return id
}

func TestNewCustomer(t *testing.T) {
	id := happyCustomer(t, "New Customer")
	defer deleteCustomer(t, id)
}

func TestCustomerAddToBalance(t *testing.T) {
	id := happyCustomer(t, "Add Balance Customer")
	defer deleteCustomer(t, id)

	amount := int64(10)
	after, err := CustomerAddToBalance(id, amount)
	if err != nil {
		t.Fatal("could not add balance to customer")
	}
	if after != amount {
		t.Fatal("new balance should be equal to the amount added", after, amount)
	}

	amount2 := int64(20)
	after2, err := CustomerAddToBalance(id, amount2)
	if err != nil {
		t.Fatal("could not add balance to customer")
	}
	if after2 != amount+amount2 {
		t.Fatal("new balance should be equal to the combination of the two ammounts", after2, amount+amount2)
	}
}

func TestCustomerSetDefaultSource(t *testing.T) {
	customerID := happyCustomer(t, "Set default card")
	defer deleteCustomer(t, customerID)

	_, err := NewCard(customerID, testCardToken)
	if err != nil {
		t.Fatal("Could not add new visa card", err)
	}
	debit, err := NewCard(customerID, testDebitToken)
	if err != nil {
		t.Fatal("Could not add new visa debit card", err)
	}

	if err := CustomerSetDefaultSource(customerID, debit.ID); err != nil {
		t.Fatal("Could not set default card")
	}
}
