package utils

import (
	"math/big"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/test/testutils/general"
)

func RandomString(length int) string {
	return general.RandomInlineString(length)
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
