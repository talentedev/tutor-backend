package stripe

import "testing"

var happyBankAccountNumber = "000123456789"
var happyBankRoutingNumber = "110000000"

func addBankAccount(t *testing.T, accountID, accountHolderName string) string {
	// https://stripe.com/docs/ach#testing-ach https://stripe.com/docs/connect/testing
	token, err := NewExternalBankAccountToken(accountHolderName, happyBankAccountNumber, happyBankRoutingNumber, "")
	if err != nil {
		t.Fatal("Could not create token", err)
	}

	ba, err := NewExternalBankAccount(accountID, token)
	if err != nil {
		t.Fatal("Could not add external bank account ", err)
	}
	return ba.ID
}

func TestNewExternalBankAccount(t *testing.T) {
	id := newAccount(t, "BankAccount")
	defer deleteAccount(t, id)

	addBankAccount(t, id, "")
}

func TestListBankAccounts(t *testing.T) {
	id := newAccount(t, "BankAccount")
	defer deleteAccount(t, id)

	addBankAccount(t, id, "")
	addBankAccount(t, id, "")

	accounts, err := ListBankAccounts(id)
	if err != nil {
		t.Fatal("Could not list back accounts", err)
	}

	if len(accounts) != 2 {
		for _, a := range accounts {
			t.Logf("%#v", a)
		}
		t.Fatal("There should have been two accounts not: ", len(accounts))
	}
}

func TestDeleteBankAccount(t *testing.T) {
	id := newAccount(t, "BankAccount Delete")
	defer deleteAccount(t, id)

	aid := addBankAccount(t, id, "")
	addBankAccount(t, id, "")

	if err := DeleteBankAccount(id, aid); err != nil {
		t.Fatal("Could not delete the account", err)
	}

	_, err := GetBankAccount(id, aid)
	if err == nil {
		t.Fatal("Should have errored trying to get an account", err)
	}

	accounts, err := ListBankAccounts(id)
	if err != nil {
		t.Fatal("Could not list accounts", err)
	}

	if len(accounts) != 1 {
		t.Fatal("Number of accounts after deletion should have been 1, not: ", len(accounts))
	}

	for _, a := range accounts {
		if a.ID == aid {
			t.Fatalf("Found deleted account in listed account ")
		}
	}
}

func TestReplaceBankAccount(t *testing.T) {
	id := newAccount(t, "BankAccount")
	defer deleteAccount(t, id)

	addBankAccount(t, id, "")

	token, err := NewExternalBankAccountToken("accountHolderName", "000111111116", happyBankRoutingNumber, "")
	if err != nil {
		t.Fatal("Could not create token", err)
	}

	ba, err := ReplaceBankAccount(id, token)
	if err != nil {
		t.Fatal("Error replacing bank account", err)
	}

	accounts, err := ListBankAccounts(id)
	if err != nil {
		t.Fatal("Could not list back accounts", err)
	}

	if len(accounts) != 1 {
		t.Fatal("There should have been two accounts not: ", len(accounts))
	}

	if accounts[0].ID != ba.ID {
		t.Fatal("Listed account id does not match the new one created", accounts[0].ID, ba.ID)
	}
}
