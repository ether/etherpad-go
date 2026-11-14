package testutils

import "embed"

//go:embed assets
var TestAssets embed.FS

func GetTestAssets() embed.FS {
	return TestAssets
}
