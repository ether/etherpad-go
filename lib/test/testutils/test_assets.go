package testutils

import "embed"

//go:embed test_assets/assets
var TestAssets embed.FS

func GetTestAssets() embed.FS {
	return TestAssets
}
