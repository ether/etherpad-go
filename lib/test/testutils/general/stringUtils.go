package general

import (
	"math/rand"
	"strings"
)

func RandomMultiline(approxMaxLines, approxMaxCols int) string {
	numParts := rand.Intn(approxMaxLines*2) + 1
	var txt strings.Builder

	if rand.Intn(2) == 1 {
		txt.WriteString("\n")
	}

	for i := 0; i < numParts; i++ {
		if i%2 == 0 {
			if rand.Intn(10) != 0 {
				txt.WriteString(RandomInlineString(rand.Intn(approxMaxCols) + 1))
			} else {
				txt.WriteString("\n")
			}
		} else {
			txt.WriteString("\n")
		}
	}

	return txt.String()
}

func RandomInlineString(length int) string {
	const chars = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789 !@#$%^&*()_+-=[]{}|;:,.<>?"
	var result strings.Builder
	result.Grow(length)

	for i := 0; i < length; i++ {
		result.WriteByte(chars[rand.Intn(len(chars))])
	}

	return result.String()
}
