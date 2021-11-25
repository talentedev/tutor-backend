package utils

import (
	"strings"
	"unicode/utf8"
)

// CapitalizeFirstWord does what its name implies.
func CapitalizeFirstWord(s string) string {
	return strings.ToUpper(string(s[0])) + string(s[1:])
}

func replaceUtf8(s string) string {
	stringRune, _ := utf8.DecodeRuneInString(s)

	chars := map[string]string{
		"ă": "a",
		"î": "i",
		"â": "a",
		"ț": "t",
		"ș": "s",
	}

	for x, y := range chars {
		charRune, _ := utf8.DecodeRuneInString(x)

		if charRune == stringRune {
			return y
		}
	}

	return s
}

// UTF8ToASCII replaces UTF-8 runes in a string with their simple counterparts.
func UTF8ToASCII(s string) string {
	var out string

	for _, c := range s {
		out += replaceUtf8(string(c))
	}

	return out
}
