package utils

import "os"

func IsDevModeEnabled() bool {
	nodeEnv := os.Getenv("NODE_ENV")
	return nodeEnv == "development" || nodeEnv == "dev"
}
