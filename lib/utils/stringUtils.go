package utils

import (
	"crypto/rand"
	"encoding/hex"
)

func RandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}
