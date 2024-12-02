package utils

import (
	"errors"
	"strconv"
)

func CheckValidRev(rev string) (*int, error) {
	var revNum, err = strconv.Atoi(rev)
	if err != nil {
		return nil, err
	}
	if revNum < 0 {
		return nil, errors.New("rev is not a negative number")
	}
	return &revNum, nil
}
