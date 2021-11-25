package stripe

//import (
//	"testing"
//)
//
// todo: readd card tests
//
//func TestNewCardToken(t *testing.T) {
//	if _, err := NewCardToken(
//		"", "", testCardNumber, "01", "2021", "123"); err != nil {
//		t.Fatal("Could not create card token", err)
//	}
//}
//
//func TestCard(t *testing.T) {
//	custID := happyCustomer(t, "Add Card")
//	defer deleteCustomer(t, custID)
//
//	card, err := NewCard(custID, testCardToken)
//	if err != nil {
//		t.Fatal("Could not add new card", err)
//	}
//
//	if _, err := GetCard(custID, card.ID); err != nil {
//		t.Fatal("could not get created card", err)
//	}
//
//	cards, err := ListCards(custID)
//	if err != nil {
//		t.Fatal("could not list cards", err)
//	}
//
//	if len(cards) != 1 {
//		t.Fatal("Only expected 1 card, not ", len(cards))
//	}
//
//	if _, err := DeleteCard(custID, card.ID); err != nil {
//		t.Fatal("could not delete card", err)
//	}
//
//	cards, err = ListCards(custID)
//	if err != nil {
//		t.Fatal("could not list cards", err)
//	}
//
//	if len(cards) != 0 {
//		t.Fatal("Only expected 0 cards, not ", len(cards))
//	}
//
//}
//
//func TestNoDuplicationNewCardToken(t *testing.T) {
//	custID := happyCustomer(t, "Dup Card")
//	defer deleteCustomer(t, custID)
//
//	token, err := NewCardToken(custID, "", testCardNumber, "1", "2020", "1234")
//	if err != nil {
//		t.Fatal("Could not create card token", err)
//	}
//	if _, err := NewCard(custID, token); err != nil {
//		t.Fatal("Could not add new card", err)
//	}
//
//	if _, err = NewCardToken(custID, "", testCardNumber, "1", "2020", "1234"); err == nil {
//		t.Fatal("the second card should have failed")
//	}
//}
