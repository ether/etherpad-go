package testutils

import (
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
)

func ArrayToPool(array []string) {
	attbPool := apool.NewAPool()
	for _, attr := range array {
		kvArr := strings.Split(attr, ",")
		if len(kvArr) == 1 {
			attbPool.PutAttrib(apool.Attribute{Key: kvArr[0], Value: ""}, nil)
		} else {
			attbPool.PutAttrib(apool.Attribute{Key: kvArr[0], Value: kvArr[1]}, nil)
		}
	}

}
