//go:build !unix && !windows

package updater

import "errors"

func diskFree(string) (uint64, error) { return 0, errors.New("unknown on this platform") }
