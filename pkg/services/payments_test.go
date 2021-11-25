package services

import (
	"testing"
	"time"
)

func TestLessonAmounts(t *testing.T) {

	tutorRate := 10.50 // Dollars per hour
	duration := (time.Hour).Minutes()

	// expectedTutor value only work if the duration is 1 hour
	expectedTutor := int64(tutorRate * 100)                              //1050
	expectedFee := int64(float64(expectedTutor) * platformFeePercentage) //315
	expectedStudent := expectedTutor + expectedFee                       //1365

	tutorPay, platformFee, studentCost := lessonAmounts(tutorRate, duration)

	if tutorPay != expectedTutor {
		t.Error("tutor pay cents didn't match ", tutorPay, expectedTutor)
	}

	if platformFee != expectedFee {
		t.Error("fee cents didn't match", platformFee, expectedFee)
	}

	if studentCost != expectedStudent {
		t.Error("student cost cents didn't match", studentCost, expectedStudent)
	}
}
