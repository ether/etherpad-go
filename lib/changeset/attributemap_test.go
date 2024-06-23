package changeset

import (
	"github.com/ether/etherpad-go/lib/apool"
	"strconv"
	"testing"
)

func PrepareAttribPool(t *testing.T) (apool.APool, [][]string) {
	var attribs = [][]string{{"foo", "bar"}, {"baz", "bif"}}
	var pool = apool.NewAPool()
	for i, attrib := range attribs {
		var nextNum = pool.PutAttrib(apool.FromJsonAble(attrib), nil)
		if i != nextNum {
			t.Error("Expected " + strconv.Itoa(i) + ", got " + strconv.Itoa(nextNum))
		}
	}
	return *pool, attribs
}

func TestSet(t *testing.T) {
	var p, _ = PrepareAttribPool(t)
	var m = NewAttributeMap(&p)
	if m.Size() != 0 {
		t.Error("Expected 0, got ", m.Size())
	}

	m.Set("foo", "bar")

	if m.Size() != 1 {
		t.Error("Expected 1, got ", m.Size())
	}

	if m.Get("foo") != "bar" {
		t.Error("Expected bar, got ", m.Get("foo"))
	}
}

func getPoolSize(t *testing.T) int {
	var n = 0
	var pool, _ = PrepareAttribPool(t)

	pool.EachAttrib(func(attrib apool.Attribute) {
		n++
	})

	return n
}

func TestReuseAttribsFromPool(t *testing.T) {
	var pool, attribs = PrepareAttribPool(t)
	if getPoolSize(t) != len(attribs) {
		t.Error("Expected ", len(attribs), ", got ", getPoolSize(t))
	}
	var m = NewAttributeMap(&pool)
	var firstset = attribs[0]
	m.Set(firstset[0], firstset[1])
	if getPoolSize(t) != len(attribs) {
		t.Error("Expected ", len(attribs)-1, ", got ", getPoolSize(t))
	}
	if m.Size() != 1 {
		t.Error("Expected 1, got ", m.Size())
	}

	if m.String() != "*0" {
		// TODO fixme this is wrong
		t.Error("Expected *0, got ", m.String())
	}
}

func TestInsertNewAttributesInThePool(t *testing.T) {
	var pool, attribs = PrepareAttribPool(t)
	var m = NewAttributeMap(&pool)
	if getPoolSize(t) != len(attribs) {
		t.Error("Expected ", len(attribs), ", got ", getPoolSize(t))
	}

	m.Set("k", "v")
	if m.Size() != len(attribs)+1 {
		t.Error("Expected ", len(attribs)+1, ", got ", getPoolSize(t))
	}
}
