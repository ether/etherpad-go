package apool

import (
	"strconv"
)

func PreparePool() APool {
	var attribs = [][]string{[]string{"foo", "bar"}, []string{"baz", "bif"}}
	var pool = NewAPool()
	for i, attrib := range attribs {
		var nextNum = pool.PutAttrib(FromJsonAble(attrib), nil)
		if i != nextNum {
			panic("Expected " + strconv.Itoa(i) + ", got " + strconv.Itoa(nextNum))
		}
	}
	return *pool
}
