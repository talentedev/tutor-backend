package utils

import (
	"testing"
)

func TestUTF8ToASCII(t *testing.T) {
	if UTF8ToASCII("ăâîțș") != "aaits" {
		t.Fail()
	}
}

func TestCapitalizeFirstWord(t *testing.T) {
	if CapitalizeFirstWord("this is a sentence.") != "This is a sentence." {
		t.Fail()
	}
}
