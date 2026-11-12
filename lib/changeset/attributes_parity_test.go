package changeset

import (
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/stretchr/testify/require"
)

// Cases of etherpad-lite's attributes spec that attributes_test.go did not
// have yet: attribsToNums and AttribsToString (reusing and inserting pool
// entries) and the accepted inputs of AttribsFromString.

var (
	attrFoo = apool.Attribute{Key: "foo", Value: "bar"}
	attrBaz = apool.Attribute{Key: "baz", Value: "bif"}
	attrNew = apool.Attribute{Key: "k", Value: "v"}
)

func twoAttribPool(t *testing.T) *apool.APool {
	t.Helper()
	pool := apool.NewAPool()
	require.Equal(t, 0, pool.PutAttrib(attrFoo, nil))
	require.Equal(t, 1, pool.PutAttrib(attrBaz, nil))
	return &pool
}

func TestAttribsToNumsAndString(t *testing.T) {
	cases := []struct {
		name    string
		in      []apool.Attribute
		nums    []int
		str     string
		inserts bool
	}{
		{"empty", nil, []int{}, "", false},
		{"first", []apool.Attribute{attrFoo}, []int{0}, "*0", false},
		{"second", []apool.Attribute{attrBaz}, []int{1}, "*1", false},
		{"both", []apool.Attribute{attrFoo, attrBaz}, []int{0, 1}, "*0*1", false},
		{"both, order kept", []apool.Attribute{attrBaz, attrFoo}, []int{1, 0}, "*1*0", false},
		{"new", []apool.Attribute{attrNew}, []int{2}, "*2", true},
		{"existing then new", []apool.Attribute{attrFoo, attrNew}, []int{0, 2}, "*0*2", true},
		{"new then existing", []apool.Attribute{attrNew, attrFoo}, []int{2, 0}, "*2*0", true},
	}
	for _, tc := range cases {
		t.Run("attribsToNums "+tc.name, func(t *testing.T) {
			pool := twoAttribPool(t)
			got := attribsToNums(tc.in, pool)
			require.Equal(t, tc.nums, append([]int{}, got...))
			checkInserted(t, pool, tc.inserts)
		})
		t.Run("AttribsToString "+tc.name, func(t *testing.T) {
			pool := twoAttribPool(t)
			got, err := AttribsToString(tc.in, pool)
			require.NoError(t, err)
			require.Equal(t, tc.str, *got)
			checkInserted(t, pool, tc.inserts)
		})
	}
}

func checkInserted(t *testing.T, pool *apool.APool, inserted bool) {
	t.Helper()
	if !inserted {
		require.Equal(t, 2, poolSize(pool), "existing entries are reused")
		return
	}
	require.Equal(t, 3, poolSize(pool))
	got, err := pool.GetAttrib(2)
	require.NoError(t, err)
	require.Equal(t, attrNew, *got)
}

func TestAttribsFromStringAcceptsValidInputs(t *testing.T) {
	cases := []struct {
		in   string
		want []apool.Attribute
	}{
		{"", []apool.Attribute{}},
		{"*0", []apool.Attribute{attrFoo}},
		{"*1", []apool.Attribute{attrBaz}},
		{"*0*1", []apool.Attribute{attrFoo, attrBaz}},
		{"*1*0", []apool.Attribute{attrBaz, attrFoo}},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			pool := twoAttribPool(t)
			got := AttribsFromString(tc.in, *pool)
			require.Equal(t, tc.want, append([]apool.Attribute{}, got...))
		})
	}
}
