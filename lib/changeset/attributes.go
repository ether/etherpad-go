package changeset

import (
	"errors"
	"github.com/ether/etherpad-go/lib/apool"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

var regex *regexp.Regexp

func init() {
	regex, _ = regexp.Compile("\\*([0-9a-z]+)|.")
}

func DecodeAttribString(s string) ([]int, error) {
	if s == "" {
		return nil, nil
	}
	var attribs []int

	matches := regex.FindAllStringSubmatch(s, -1)

	for _, match := range matches {
		if len(match) != 2 {
			return nil, errors.New("invalid match")
		}
		if match[1] == "" {
			return nil, errors.New("invalid character in attribute string: " + match[0])
		}
		num, err := strconv.ParseInt(match[1], 36, 0)

		if err != nil {
			return nil, err
		}
		attribs = append(attribs, int(num))
	}

	return attribs, nil
}

func checkAttribNum(n int) error {
	if n < 0 {
		return errors.New("Attrib number is negative")
	}
	return nil
}

func encodeAttribString(attribNums []int) string {
	var str string
	for _, num := range attribNums {
		str += "*" + strings.ToLower(strconv.FormatInt(int64(num), 36))
	}
	return str
}

func attribsFromNums(attribNums []int, pool *apool.APool) []apool.Attribute {
	var attribs []apool.Attribute
	for _, num := range attribNums {
		attribs = append(attribs, pool.GetAttrib(num))
	}
	return attribs
}

func attribsToNums(attribs []apool.Attribute, pool *apool.APool) []int {
	var nums []int
	for _, attrib := range attribs {
		nums = append(nums, pool.PutAttrib(attrib, nil))
	}
	return nums
}

func AttribsFromString(str string, pool *apool.APool) []apool.Attribute {
	attribNums, err := DecodeAttribString(str)
	if err != nil {
		return nil
	}
	return attribsFromNums(attribNums, pool)
}

func AttribsToString(attribs []apool.Attribute, pool *apool.APool) string {
	return encodeAttribString(attribsToNums(attribs, pool))
}

func SortAttribs(attribs []apool.Attribute) {
	slices.SortFunc(attribs, apool.CmpAttribute)
}
