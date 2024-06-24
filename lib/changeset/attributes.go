package changeset

import (
	"errors"
	"github.com/ether/etherpad-go/lib/apool"
	"regexp"
	"strconv"
	"strings"
)

var regex *regexp.Regexp

func init() {
	regex, _ = regexp.Compile("\\*([0-9a-z]+)|.")
}

func StringToAttrib(attrib []string) (*apool.Attribute, error) {

	if len(attrib) != 2 {
		return nil, errors.New("invalid attribute")
	}

	return &apool.Attribute{Key: attrib[0], Value: attrib[1]}, nil
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

func encodeAttribString(attribNums []int) (*string, error) {
	var str string
	for _, num := range attribNums {
		var encodedInt = int64(num)

		if encodedInt < 0 {
			return nil, errors.New("Attrib number is negative")
		}

		str += "*" + strings.ToLower(strconv.FormatInt(encodedInt, 36))
	}
	return &str, nil
}

func attribsFromNums(attribNums []int, pool apool.APool) (*[]apool.Attribute, error) {
	var attribs []apool.Attribute
	for _, num := range attribNums {
		if num < 0 {
			return nil, errors.New("attrib number is negative")
		}
		attrib, err := pool.GetAttrib(num)
		if err != nil {
			return nil, err
		}

		attribs = append(attribs, *attrib)
	}
	return &attribs, nil
}

func attribsToNums(attribs []apool.Attribute, pool *apool.APool) []int {
	var nums []int
	for _, attrib := range attribs {
		nums = append(nums, pool.PutAttrib(attrib, nil))
	}
	return nums
}

func AttribsFromString(str string, pool apool.APool) []apool.Attribute {
	attribNums, err := DecodeAttribString(str)
	if err != nil {
		return nil
	}
	attribs, err := attribsFromNums(attribNums, pool)
	if err != nil {
		return nil
	}
	return *attribs
}

func AttribsToString(attribs []apool.Attribute, pool *apool.APool) (*string, error) {
	return encodeAttribString(attribsToNums(attribs, pool))
}
