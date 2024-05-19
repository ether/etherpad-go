package utils

import (
	"crypto/rand"
	"encoding/hex"
	"math/big"
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
