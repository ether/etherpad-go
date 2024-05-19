package pad

import (
	"errors"
	"github.com/ether/etherpad-go/lib/models/pad"
	"regexp"
	"strconv"
	"strings"
)

func DecodeAttribString(str string) (<-chan int, error) {
	ch := make(chan int)
	re := regexp.MustCompile(`\*([0-9a-z]+)|.`)

	go func() {
		defer close(ch)

		matches := re.FindAllStringSubmatch(str, -1)
		for _, match := range matches {
			if len(match) < 2 || match[1] == "" {
				close(ch)
				return
			}

			n, err := strconv.ParseInt(match[1], 36, 0)
			if err != nil {
				close(ch)
				return
			}

			ch <- int(n)
		}
	}()

	return ch, nil
}

func checkAttribNum(n int) error {
	if n < 0 {
		return errors.New("Attrib number is negative")
	}
	return nil
}

func EncodeAttribString(attribNums []int) (*string, error) {
	var str string
	str = ""
	for _, n := range attribNums {
		err := checkAttribNum(n)
		if err != nil {
			return nil, err
		}

		str += "*" + strings.ToLower(strconv.FormatInt(int64(n), 36))
	}

	return &str, nil
}

func attribsFromNums(attribNums []int, pool *pad.APool) (<-chan pad.Attribute, error) {
	ch := make(chan pad.Attribute)

	go func() {
		defer close(ch)

		for _, n := range attribNums {
			err := checkAttribNum(n)
			if err != nil {
				return
			}

			attrib := pool.GetAttrib(n)

			ch <- attrib
		}
	}()

	return ch, nil
}

func attribsFromString(str string, pool *pad.APool) (<-chan pad.Attribute, error) {
	ch := make(chan pad.Attribute)

	go func() {
		defer close(ch)

		nums, err := DecodeAttribString(str)
		if err != nil {
			return
		}

		for num := range nums {
			attribs, err := attribsFromNums(num, pool)
			if err != nil {
				return
			}

			for attrib := range attribs {
				ch <- attrib
			}
		}
	}()

	return ch, nil
}
