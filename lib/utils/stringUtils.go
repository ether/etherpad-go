package utils

import (
	"crypto/rand"
	"math/big"
	"strconv"
	"strings"
)

const randomStringChars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

// RandomString returns length characters from [A-Za-z0-9], drawn from
// crypto/rand. Its output is used for secrets (author tokens, session IDs,
// read-only pad IDs), so it must not come from math/rand (cf. etherpad-lite's
// crypto.randomBytes-based randomString).
func RandomString(length int) string {
	// Rejection sampling: 248 = 4*62 is the largest multiple of the alphabet
	// size that fits a byte, so every character is equally likely.
	const limit = 256 - 256%len(randomStringChars)
	var sb strings.Builder
	sb.Grow(length)
	buf := make([]byte, length+length/4+8)
	for sb.Len() < length {
		if _, err := rand.Read(buf); err != nil {
			// crypto/rand.Read never fails on supported platforms (Go 1.24+).
			panic("crypto/rand unavailable: " + err.Error())
		}
		for _, b := range buf {
			if int(b) >= limit {
				continue
			}
			sb.WriteByte(randomStringChars[int(b)%len(randomStringChars)])
			if sb.Len() == length {
				break
			}
		}
	}
	return sb.String()
}

// GenerateAuthorToken returns a fresh author token ("t." + 20 random
// characters), matching padutils.generateAuthorToken in etherpad-lite.
func GenerateAuthorToken() string {
	return "t." + RandomString(20)
}

func NumToString(num int) string {
	return strings.ToLower(big.NewInt(int64(num)).Text(36))
}

func ParseNum(num string) (int, error) {
	var res, err = strconv.ParseInt(num, 36, 0)
	if err != nil {
		return 0, err
	}
	return int(res), nil
}

func CountLines(s string, r rune) int {
	count := 0
	for _, c := range s {
		if c == r {
			count++
		}
	}
	return count
}

func CountLinesRunes(s []rune, r rune) int {
	count := 0
	for _, c := range s {
		if c == r {
			count++
		}
	}
	return count
}

func EndsWithNewLine(s []rune) bool {
	if len(s) == 0 {
		return false
	}
	return s[len(s)-1] == '\n'
}
