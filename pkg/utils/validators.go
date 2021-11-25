package utils

import (
	"net"
	"net/mail"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gitlab.com/learnt/api/config"
)

// IsValidEmailAddress checks if a string is a valid email address.
func IsValidEmailAddress(e string) bool {
	// parse the address
	addr, err := mail.ParseAddress(e)
	if err != nil {
		return false
	}
	// check for valid format
	split := strings.Split(addr.Address, "@")
	if len(split) != 2 {
		return false
	}
	// lookup host
	_, err = net.LookupHost(split[1])
	if err != nil {
		return false
	}
	return true
}

// IsValidPassword checks if a string is a valid password.
// Checks for a minimum length of 8 characters, at least a lowercase character,
// at least an uppercase character, and at least a digit.
func IsValidPassword(p string) bool {
	// minimum length
	if len(p) < 8 {
		return false
	}
	// check for uppercase, lowercase, digits
	for _, testRune := range []func(rune) bool{
		unicode.IsUpper,
		unicode.IsLower,
		unicode.IsDigit,
	} {
		found := false
		for _, r := range p {
			if testRune(r) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// IsValidUSBankAccountNumber checks if a string is a valid US bank account
// number. There's no clear standard, so we check for 4 to 17 digits, or,
// an alphanumeric string of 4 to 17 characters.
func IsValidUSBankAccountNumber(n string, alphaNum bool) bool {
	if len(n) < 4 || len(n) > 17 {
		return false
	}
	re := "^[0-9]{4,17}$"
	if alphaNum {
		re = "^[A-Z0-9]{4,17}$"
	}
	match, _ := regexp.MatchString(re, strings.ToUpper(n))
	return match
}

// IsValidUSBankRoutingNumber checks if a string is a valid US bank routing number.
// Standard says 9 digit string, additionally check for validity based on BrainTree's
// npm module: https://github.com/braintree/us-bank-account-validator
func IsValidUSBankRoutingNumber(n string) bool {
	if len(n) != 9 {
		return false
	}
	for _, v := range n {
		if !unicode.IsDigit(v) {
			return false
		}
	}
	for _, v := range routingNumberList {
		if v == n {
			return true
		}
	}

	if strings.HasPrefix(config.GetConfig().GetString("service.stripe.key"), "pk_test_") {
		if n == "110000000" {
			return true
		}
	}

	return false
}

// IsValidPattern checks a string on a provided RegExp.
func IsValidPattern(r, s string) bool {
	re, err := regexp.Compile(r)
	if err != nil {
		return false
	}
	return re.MatchString(s)
}

// AreMutualExclusive returns whether two strings are mutual exclusive or not.
// More precisely, checks if only one of them is empty, while other is not.
func AreMutualExclusive(a, b string) bool {
	switch true {
	case a == "" && b != "", a != "" && b == "":
		return true
	default:
		return false
	}
}

// IsStringTime checks if a string is a valid time entry in string format.
// Checks for times of format (0)0:00 am|pm.
func IsStringTime(t string, twelveHourClock bool) bool {
	reg := `^[0-9]{1,2}:[0-9]{1,2}$`
	if twelveHourClock {
		reg = `^[0-9]{1,2}:[0-9]{1,2} (a|p)m$`
	}

	re, _ := regexp.MatchString(reg, t)
	if !re {
		return false
	}

	sepPos := strings.Index(t, ":")
	hour, err := strconv.Atoi(t[0:sepPos])
	if err != nil {
		return false
	}

	minute, err := strconv.Atoi(t[sepPos+1 : sepPos+3])
	if err != nil {
		return false
	}

	if hour == 24 && minute > 0 {
		return false
	}

	if hour > 24 {
		return false
	}

	if twelveHourClock && hour > 12 {
		return false
	}

	if minute > 59 {
		return false
	}

	return true
}

// IsSpecificTime checks if a string is a valid specific time, following
// the format 'year-month-day_from:time_to:time'. Example '2018-04-30_04:30_08:00'.
func IsSpecificTime(t string) bool {
	if len(t) != 22 {
		// 4 year, 2 month, 2 day, 8 hours & mins, 6 separators
		return false
	}

	m, err := regexp.MatchString(`[0-9]{4}-[0-9]{2}-[0-9]{2}_[0-9]{2}:[0-9]{2}_[0-9]{2}:[0-9]{2}`, t)
	if err != nil || !m {
		return false
	}

	date := t[0:10]
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return false
	}

	month, err := strconv.Atoi(date[5:7])
	if err != nil || month > 12 {
		return false
	}

	day, err := strconv.Atoi(date[8:])
	if err != nil || day > 31 {
		return false
	}

	fromTime := t[11:16]
	toTime := t[17:]
	if !IsStringTime(fromTime, false) || !IsStringTime(toTime, false) {
		return false
	}

	return true
}

// IsSSN checks if a string is a valid Social Security Number.
// http://rion.io/2013/09/10/validating-social-security-numbers-through-regular-expressions-2/
func IsSSN(ssn string) bool {
	if len(ssn) != 11 {
		// 9 digits + 2 dashes
		return false
	}

	if ssn == "219-09-9999" || ssn == "078-05-1120" {
		// public examples or retired SSNs
		return false
	}

	m, err := regexp.MatchString(`^\d{3}-\d{2}-\d{4}$`, ssn)
	if err != nil || !m {
		// basic format check
		return false
	}

	if ssn[:3] == "666" || ssn[:3] == "000" {
		return false
	}

	area, err := strconv.Atoi(ssn[:3])
	if err != nil {
		return false
	}

	if area >= 900 && area <= 999 {
		// invalid area
		return false
	}

	if ssn[4:6] == "00" {
		// invalid group
		return false
	}

	if ssn[7:] == "0000" {
		// invalid serial number
		return false
	}

	return true
}
