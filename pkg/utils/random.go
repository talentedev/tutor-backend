package utils

import (
	"encoding/base64"
	"encoding/hex"
	"math/rand"
	"time"
)

// charset used to generate a random string
const letterBytes = "1234567890abcdefghijklmnopqrstuvwxyz"

const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

// RandString generates a random string containing n characters of the charset letterBytes.
// https://stackoverflow.com/questions/22892120/how-to-generate-a-random-string-of-a-fixed-length-in-golang
func RandString(n int) string {
	src := rand.NewSource(time.Now().UnixNano())
	b := make([]byte, n)
	// A src.Int63() generates 63 random bits, enough for letterIdxMax characters!
	for i, cache, remain := n-1, src.Int63(), letterIdxMax; i >= 0; {
		if remain == 0 {
			cache, remain = src.Int63(), letterIdxMax
		}
		if idx := int(cache & letterIdxMask); idx < len(letterBytes) {
			b[i] = letterBytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}

// RandToken returns l random bytes encoded in base64.
func RandToken(l int) string {
	b := make([]byte, l)
	rand.Read(b)
	return base64.StdEncoding.EncodeToString(b)
}

// RandBytes returns l random bytes
func RandBytes(l int) []byte {
	b := make([]byte, l)
	rand.Read(b)
	return b
}

func Hex(n int) string { return hex.EncodeToString(RandBytes(n)) }
