package utils

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
	"strconv"
	"strings"
)

func RandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
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
