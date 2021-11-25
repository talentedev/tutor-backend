package utils

import "testing"

func TestIsEmailAddress(t *testing.T) {
	tests := []struct {
		name     string
		email    string
		expected bool
	}{
		{"regular", "john_doe@gmail.com", true},
		{"named", "my.email+provider@yahoo.com", true},
		{"incorrect", "email@address", false},
		{"can't lookup", "false@false.false", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if IsValidEmailAddress(test.email) != test.expected {
				t.Error(t.Name())
			}
		})
	}
}

func TestIsValidPassword(t *testing.T) {
	tests := []struct {
		name     string
		password string
		expected bool
	}{
		{"short", "mypass", false},       // fails length
		{"simple", "password", false},    // fails uppercase & digit
		{"mixed", "PassWord", false},     // fails digit
		{"correct", "PassWord123", true}, // passes tests but it's bad
		{"overkill", "WmCAkNJ!82#zBpanNs", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if IsValidPassword(test.password) != test.expected {
				t.Error(test.name)
			}
		})
	}
}

func TestIsValidUSBankAccountNumber(t *testing.T) {
	tests := []struct {
		name     string
		number   string
		alphaNum bool
		expected bool
	}{
		{"valid", "123456", false, true},
		{"valid", "123456ABC", true, true},
		{"too short", "123", false, false},
		{"too short", "abc", true, false},
		{"too long", "12345678901234567890", false, false},
		{"too long", "1234567890abcdefabcd", true, false},
		{"alpha", "1234abc", false, false},
		{"symbols", "123!@#", true, false},
		{"symbols", "123!@#", false, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if IsValidUSBankAccountNumber(test.number, test.alphaNum) != test.expected {
				t.Error(test.name)
			}
		})
	}
}

func TestIsValidUSBankRoutingNumber(t *testing.T) {
	tests := []struct {
		name     string
		number   string
		expected bool
	}{
		{"valid", "101000019", true},
		{"valid", "281082038", true},
		{"valid", "322270770", true},
		{"valid", "051502159", true},
		{"invalid", "000000000", false},
		{"too short", "000", false},
		{"too long", "1234567890", false},
		{"alpha", "abcdefghi", false},
		{"alphanum", "abcdef123", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if IsValidUSBankRoutingNumber(test.number) != test.expected {
				t.Error(test.name)
			}
		})
	}
}

func TestIsValidPattern(t *testing.T) {
	tests := []struct {
		name     string
		regex    string
		text     string
		expected bool
	}{
		{"simple", `^foo`, "foobar", true},
		{"simple", `.foo\s+$`, "qfoo	 ", true},
		{"simple", `^[0-9]{2,}[a-z]{1}[.,]$`, "123y,", true},
		{"simple", `^[0-9]{2,}[a-z]{1}[.,]$`, "123R!", false},
		{"incorrect regexp", `(a`, "test", false},
		{"no match", `[0-9]+`, "", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if IsValidPattern(test.regex, test.text) != test.expected {
				t.Error(test.name)
			}
		})
	}
}

func TestAreMutualExclusive(t *testing.T) {
	tests := []struct {
		name     string
		first    string
		second   string
		expected bool
	}{
		{"a, !b", "first", "", true},
		{"!a, b", "", "second", true},
		{"!a, !b", "", "", false},
		{"a, b", "first", "second", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if AreMutualExclusive(test.first, test.second) != test.expected {
				t.Error(test.name)
			}
		})
	}
}

func TestIsStringTime(t *testing.T) {
	tests := []struct {
		name       string
		time       string
		twelveHour bool
		expected   bool
	}{
		{"24 correct short", "3:33", false, true},
		{"24 correct short", "05:55", false, true},
		{"24 correct long", "13:33", false, true},
		{"24 correct hour", "24:00", false, true},
		{"24 incorrect hour", "24:01", false, false},
		{"24 incorrect hour", "25:00", false, false},
		{"24 incorrect minute", "23:65", false, false},
		{"24 incorrect format", " 23:65 pm", false, false},
		{"12 correct short", "1:00 pm", true, true},
		{"12 correct short", "06:00 am", true, true},
		{"12 correct long", "10:00 am", true, true},
		{"12 incorrect hour", "13:00 am", true, false},
		{"12 incorrect minute", "10:70 am", true, false},
		{"12 incorrect format", "1200 am", true, false},
		{"12 incorrect format", "1200am", true, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if IsStringTime(test.time, test.twelveHour) != test.expected {
				t.Error(test.name)
			}
		})
	}
}

func TestIsSpecificTime(t *testing.T) {
	tests := []struct {
		name     string
		time     string
		expected bool
	}{
		{"correct", "2018-04-30_04:30_08:00", true},
		{"correct", "2018-04-30_23:00_24:00", true},
		{"correct", "1970-01-31_00:00_24:00", true},
		{"incorrect length", "1970-01-31_0:00_24:00", false},
		{"incorrect format", "1970_04-30/04:20+22:33", false},
		{"incorrect day", "1970-01-32_00:00_24:00", false},
		{"incorrect month", "1970-13-31_00:00_24:00", false},
		{"incorrect date", "1970-02-30_00:00_24:00", false},
		{"incorrect hour", "1970-02-30_00:00_25:00", false},
		{"incorrect minute", "1970-02-30_00:60_24:00", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if IsSpecificTime(test.time) != test.expected {
				t.Error(test.name)
			}
		})
	}
}
