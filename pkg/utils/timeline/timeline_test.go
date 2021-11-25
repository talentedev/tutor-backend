package timeline

import (
	"bytes"
	"fmt"
	"testing"
	"time"
)

func nt(d, h, m int) time.Time {
	return time.Date(2020, time.January, d, h, m, 0, 0, time.UTC)
}
func printSlots(slots []SlotProvider) string {
	s := bytes.NewBufferString("")
	layout := "Jan 2 15:04"
	for _, slot := range slots {
		s.WriteString(fmt.Sprintf(
			"Slot from %s to %s\n",
			slot.GetFrom().Format(layout),
			slot.GetTo().Format(layout),
		))
	}
	return s.String()
}

func TestIsBetween(t *testing.T) {

	if IsBetween(nt(1, 13, 0), nt(1, 10, 0), nt(1, 12, 0)) {
		t.Error("Failed")
	}

	if !IsBetween(nt(1, 10, 0), nt(1, 10, 0), nt(1, 12, 0)) {
		t.Error("Failed")
	}

	if !IsBetween(nt(1, 12, 0), nt(1, 10, 0), nt(1, 12, 0)) {
		t.Error("Failed")
	}

	if IsBetween(nt(1, 12, 1), nt(1, 10, 0), nt(1, 12, 0)) {
		t.Error("Failed")
	}

	if IsBetween(nt(1, 0, 59), nt(1, 10, 0), nt(1, 12, 0)) {
		t.Error("Failed")
	}
}

func TestMerge(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: Weekly,
	}

	b := &Slot{
		From:      nt(1, 12, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	expected := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	line := Timeline{}
	line.Add(a, b)

	out := line.at(nt(1, 11, 0))

	if out == nil {
		t.Error("Slot expected")
		return
	}

	if out.Equal(a) {
		t.Error("Expected not equal")
		return
	}

	if !out.Equal(expected) {
		t.Error("Expected equal")
		return
	}

	if line.Len() != 1 {
		t.Error("Expected one slot")
	}
}
func TestSubTimeline(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: Weekly,
	}

	b := &Slot{
		From:      nt(1, 12, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	merged := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	line := Timeline{}
	line.Add(a, b)

	if len(line.slots) != 1 {
		t.Error("Expected to have one slot due merge")
		return
	}

	if !line.slots[0].Equal(merged) {
		t.Error("Expected merged slot match the model")
		return
	}

	diff := &Slot{
		From:      nt(1, 11, 0),
		To:        nt(1, 12, 0),
		Occurence: Weekly,
	}

	out, _ := line.Get(nt(1, 10, 0), nt(1, 13, 0), diff)

	if len(out) != 2 {
		t.Errorf("Expected 2 slot, has %d", len(out))
		t.Log(printSlots(out))
	}

	splitA := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 11, 0),
		Occurence: Weekly,
	}

	splitB := &Slot{
		From:      nt(1, 12, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	if !out[0].Equal(splitA) {
		t.Errorf("Expected slot match splitA")
		return
	}

	if !out[1].Equal(splitB) {
		t.Errorf("Expected slot match splitB")
		return
	}
}

func TestVerifyShift(t *testing.T) {
	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 11, 0),
		Occurence: Weekly,
	}

	line := Timeline{}
	line.Add(a)
	out, _ := line.Get(nt(1, 10, 0), nt(10, 12, 0))

	if len(out) != 2 {
		t.Errorf("Expected two slots, found %d", len(out))
	}
}

func TestSubtract(t *testing.T) {
	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	diff := &Slot{
		From:      nt(1, 11, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	line := Timeline{}
	line.Add(a)
	out, _ := line.Get(nt(1, 10, 0), nt(10, 12, 0), diff)

	if len(out) != 3 {
		t.Errorf("Expected 3 slots, found %d", len(out))
	}
}

func TestSubtractExtendedRange(t *testing.T) {
	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	diffa := &Slot{
		From:      nt(1, 11, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	diffb := &Slot{
		From:      nt(8, 11, 0),
		To:        nt(8, 12, 0),
		Occurence: None,
	}

	line := Timeline{}
	line.Add(a)
	out, _ := line.Get(nt(1, 10, 0), nt(16, 12, 0), diffa, diffb)

	if len(out) != 5 {
		t.Errorf("Expected 5 slots, found %d", len(out))
	}
}

func TestSubtractRecurrentDiff(t *testing.T) {
	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	diffa := &Slot{
		From:      nt(1, 11, 0),
		To:        nt(1, 12, 0),
		Occurence: Weekly,
	}

	diffb := &Slot{
		From:      nt(8, 11, 0),
		To:        nt(8, 12, 0),
		Occurence: Weekly,
	}

	line := Timeline{}
	line.Add(a)
	out, _ := line.Get(nt(1, 10, 0), nt(16, 12, 0), diffa, diffb)

	if len(out) != 6 {
		t.Errorf("Expected 6 slots, found %d", len(out))
	}
}

func TestDiffAll(t *testing.T) {
	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	diffa := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 11, 0),
		Occurence: Weekly,
	}

	diffb := &Slot{
		From:      nt(1, 11, 0),
		To:        nt(1, 12, 0),
		Occurence: Weekly,
	}

	diffc := &Slot{
		From:      nt(1, 12, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	line := Timeline{}
	line.Add(a)
	out, _ := line.Get(nt(1, 10, 0), nt(16, 12, 0), diffa, diffb, diffc)

	if len(out) != 0 {
		t.Errorf("Expected 0 slots, found %d", len(out))
	}
}

func TestDiffAfterRange(t *testing.T) {
	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	diff := &Slot{
		From:      nt(2, 10, 0),
		To:        nt(2, 11, 0),
		Occurence: Weekly,
	}

	line := Timeline{}
	line.Add(a)
	out, _ := line.Get(nt(1, 10, 0), nt(1, 13, 0), diff)

	if len(out) != 1 {
		t.Errorf("Expected 1 slots, found %d", len(out))
	}
}

func TestNoneRecurrent(t *testing.T) {
	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: None,
	}

	diff := &Slot{
		From:      nt(2, 10, 0),
		To:        nt(2, 11, 0),
		Occurence: Weekly,
	}

	line := Timeline{}
	line.Add(a)
	out, _ := line.Get(nt(1, 10, 0), nt(1, 13, 0), diff)

	if len(out) != 1 {
		t.Errorf("Expected 1 slots, found %d", len(out))
	}
}

func TestInvalidRange(t *testing.T) {
	line := Timeline{}
	_, err := line.Get(nt(2, 10, 0), nt(1, 11, 0))

	if err == nil {
		t.Error("Expected an error of invalid range")
	}
}

func TestOutOfRange(t *testing.T) {

	a := &Slot{
		From:      nt(5, 10, 0),
		To:        nt(5, 13, 0),
		Occurence: None,
	}

	line := Timeline{}
	line.Add(a)
	out, _ := line.Get(nt(1, 10, 0), nt(1, 11, 0))

	if len(out) != 0 {
		t.Errorf("Expected 0 slots, found %d", len(out))
	}
}

func TestErrorSlotExists(t *testing.T) {

	a := &Slot{
		From:      nt(5, 10, 0),
		To:        nt(5, 13, 0),
		Occurence: None,
	}

	line := Timeline{}
	err := line.Add(a, a)

	if err == nil {
		t.Error("Expected error of slot exists")
	}
}

func TestErrorInvlidSlotRange(t *testing.T) {

	a := &Slot{
		From:      nt(2, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: None,
	}

	line := Timeline{}
	err := line.Add(a)

	if err == nil {
		t.Error("Expected invalid slot range error")
	}
}

func TestMergeInvalidSlot(t *testing.T) {

	a := &Slot{
		From:      nt(2, 10, 0),
		To:        nt(1, 11, 0),
		Occurence: None,
	}

	b := &Slot{
		From:      nt(1, 9, 0),
		To:        nt(1, 10, 30),
		Occurence: None,
	}

	line := Timeline{}
	err := line.Add(a, b)

	if err == nil {
		t.Error("Expected invalid slot error")
	}
}

func TestMergeSlotAlreadyExist(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	b := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	line := Timeline{}
	err := line.Add(a, b)

	if err == nil {
		t.Error("Expected slot already exist error")
	}
}

func TestMergeMismatchingOccurence(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	b := &Slot{
		From:      nt(1, 11, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	line := Timeline{}
	err := line.Add(a, b)

	if err == nil {
		t.Error("Expected mismatch occurence")
	}
}

func TestMergeMismatchingOccurence2(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	b := &Slot{
		From:      nt(1, 11, 0),
		To:        nt(1, 12, 0),
		Occurence: Weekly,
	}

	line := Timeline{}
	err := line.Add(a, b)

	if err == nil {
		t.Error("Expected slot found with different occurence error")
	}
}

func TestMergeMismatchingOccurence3(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	b := &Slot{
		From:      nt(1, 9, 0),
		To:        nt(1, 11, 0),
		Occurence: Weekly,
	}

	line := Timeline{}
	err := line.Add(a, b)

	if err == nil {
		t.Error("Expected slot missmatching occurence")
	}
}

func TestMergeNextSlotBeforeExisting(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	b := &Slot{
		From:      nt(1, 9, 0),
		To:        nt(1, 11, 0),
		Occurence: None,
	}

	line := Timeline{}
	line.Add(a, b)

	if len(line.slots) != 1 {
		t.Error("Expected to merge")
	}
}

func TestShiftChangeRecursive(t *testing.T) {
	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	slot := Shift(a, nt(22, 0, 0), nt(23, 0, 0))

	if slot.GetFrom().Day() != 22 {
		t.Error("Expected shift to date")
	}
}

func TestShiftReturnNilAfterPeriod(t *testing.T) {
	a := &Slot{
		From:      nt(3, 10, 0),
		To:        nt(3, 12, 0),
		Occurence: None,
	}

	slot := Shift(a, nt(1, 0, 0), nt(2, 0, 0))

	if slot != nil {
		t.Error("Expected nil slot")
	}
}

func TestErrorNoSlotsToAdd(t *testing.T) {

	line := Timeline{}
	err := line.Add()

	if err == nil {
		t.Error("Expected error no slots to add")
	}
}

func TestSplitNotInSlot(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	b := &Slot{
		From:      nt(1, 12, 0),
		To:        nt(1, 13, 0),
		Occurence: None,
	}

	out := split(a, b)

	if len(out) != 0 {
		t.Error("Expected return empty list")
	}
}

func TestRealScenario(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: None,
	}

	b := &Slot{
		From:      nt(2, 10, 0),
		To:        nt(2, 12, 0),
		Occurence: None,
	}

	c := &Slot{
		From:      nt(10, 10, 0),
		To:        nt(10, 12, 0),
		Occurence: None,
	}

	diff := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 12, 0),
		Occurence: Weekly,
	}

	line := &Timeline{}
	line.Add(a, b, c)

	out, _ := line.Get(nt(1, 10, 0), nt(3, 10, 0), diff)

	if len(out) != 1 {
		t.Error("Expected one slot")
		return
	}
}

func TestSort(t *testing.T) {

	a := &Slot{
		From:      nt(1, 12, 0),
		To:        nt(1, 13, 0),
		Occurence: None,
	}

	b := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 11, 0),
		Occurence: None,
	}

	line := &Timeline{}
	line.Add(a, b)

	if len(line.slots) != 2 {
		t.Errorf("Expected 0 slots, found %d", len(line.slots))
		return
	}

	if !line.slots[0].Equal(b) {
		t.Errorf("Expected to sort")
		return
	}
}

func TestShouldGetSlotWhenGettingExact(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: None,
	}

	line := &Timeline{}
	line.Add(a)
	out, _ := line.Get(a.From, a.To)

	if len(out) == 0 {
		t.Error("Expecting available")
	}
}

func TestAvailability(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: None,
	}

	booked := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 11, 0),
		Occurence: None,
	}

	line := &Timeline{}
	line.Add(a)

	av := line.GetAvailability(booked)

	isAvailable := av.IsAvailable(
		nt(1, 11, 0),
		nt(1, 12, 0),
	)

	if !isAvailable {
		t.Error("Expecting available")
	}
}

func TestAvailabilityRecurrentNotAvailable(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	booked := &Slot{
		From:      nt(15, 10, 0),
		To:        nt(15, 13, 0),
		Occurence: Weekly,
	}

	line := &Timeline{}
	line.Add(a)

	av := line.GetAvailability(booked)

	isAvailable := av.IsAvailableRecurrent(
		nt(8, 10, 0),
		nt(8, 13, 0),
	)

	if isAvailable {
		t.Error("Expecting not available")
	}
}

func TestAvailabilityRecurrentHalfAvailableHalfNot(t *testing.T) {

	a := &Slot{
		From:      nt(1, 1, 0),
		To:        nt(1, 10, 0),
		Occurence: Weekly,
	}

	booked := &Slot{
		From:      nt(15, 5, 0),
		To:        nt(15, 7, 0),
		Occurence: Weekly,
	}

	line := &Timeline{}
	line.Add(a)

	av := line.GetAvailability(booked)

	if av.IsAvailableRecurrent(
		nt(1, 6, 0),
		nt(1, 8, 0),
	) {
		t.Error("Expecting not available")
	}

	slots, err := av.Get(nt(15, 5, 0), nt(15, 6, 0))

	if err != nil || len(slots) != 0 {
		t.Error("Expected no slots")
		return
	}

	slots, err = av.Get(nt(1, 1, 0), nt(1, 2, 0))

	if err != nil || len(slots) != 1 {
		t.Error("Expected one slots")
		return
	}

	slots, err = av.Get(nt(15, 1, 0), nt(15, 2, 0))

	if err != nil || len(slots) != 1 {
		t.Error("Expected one slots")
		return
	}

	if av.IsAvailableRecurrent(
		nt(1, 5, 0),
		nt(1, 6, 0),
	) {
		t.Error("Expecting not available")
	}

	if !av.IsAvailableRecurrent(
		nt(1, 1, 0),
		nt(1, 3, 0),
	) {
		t.Error("Expecting available")
	}

	if av.IsAvailableRecurrent(
		nt(1, 4, 0),
		nt(1, 6, 0),
	) {
		t.Error("Expecting available")
	}
}

func TestAvailabilityRecurrent(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: Weekly,
	}

	booked := &Slot{
		From:      nt(15, 10, 0),
		To:        nt(15, 11, 0),
		Occurence: Weekly,
	}

	line := &Timeline{}
	line.Add(a)

	av := line.GetAvailability(booked)

	if av.IsAvailableRecurrent(
		nt(8, 10, 0),
		nt(8, 13, 0),
	) {
		t.Error("Expecting not available")
	}

	if !av.IsAvailableRecurrent(
		nt(8, 11, 0),
		nt(8, 12, 0),
	) {
		t.Error("Expecting available")
	}

}

func TestSlotShift(t *testing.T) {

	zone, er := time.LoadLocation("America/Louisville")
	if er != nil {
		t.Error("Invalid timezone")
		return
	}

	slot := &Slot{
		From:      time.Date(2020, time.May, 18, 13, 0, 0, 0, zone),
		To:        time.Date(2020, time.May, 18, 16, 0, 0, 0, zone),
		Occurence: Weekly,
	}

	from := time.Date(2020, time.May, 18, 0, 0, 0, 0, zone)
	to := time.Date(2020, time.May, 25, 23, 0, 0, 0, zone)

	out := Shift(slot, from, to)

	expected := &Slot{
		From:      time.Date(2020, time.May, 25, 13, 0, 0, 0, zone),
		To:        time.Date(2020, time.May, 25, 16, 0, 0, 0, zone),
		Occurence: Weekly,
	}

	if !out.Equal(expected) {
		t.Error("Failed to match")
	}
}

func TestSlotEnters(t *testing.T) {

	a := &Slot{
		From:      nt(1, 10, 0),
		To:        nt(1, 13, 0),
		Occurence: None,
	}

	if !SlotEnters(a, nt(1, 11, 0), nt(1, 12, 0)) {
		t.Error("Failed")
	}

	slot := &Slot{
		From: nt(1, 10, 0),
		To:   nt(1, 11, 0),
	}

	if !SlotEnters(slot, nt(1, 10, 0), nt(1, 13, 0)) {
		t.Error("Failed")
	}

	if !SlotEnters(slot, nt(1, 10, 0), nt(1, 11, 0)) {
		t.Error("Failed")
	}

	if SlotEnters(slot, nt(1, 9, 0), nt(1, 10, 0)) {
		t.Error("Failed")
	}

	if SlotEnters(slot, nt(1, 11, 0), nt(1, 12, 0)) {
		t.Error("Failed")
	}

	if !SlotEnters(slot, nt(1, 9, 0), nt(1, 10, 30)) {
		t.Error("Failed")
	}

	if !SlotEnters(slot, nt(1, 10, 30), nt(1, 11, 30)) {
		t.Error("Failed")
	}

	if !SlotEnters(slot, nt(1, 10, 30), nt(1, 11, 0)) {
		t.Error("Failed")
	}

}
