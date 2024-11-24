package utils

import (
	"os"
)

func ExistsSync(path string) bool {
	_, err := os.Stat(path)

	return !os.IsNotExist(err)
}
