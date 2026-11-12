package changeset

import (
	"sort"
	"testing"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/stretchr/testify/require"
)

// Cases of etherpad-lite's AttributeMap spec that attributemap_test.go did
// not have yet: String() and Update / UpdateFromString.

var parityAttribs = []apool.Attribute{
	{Key: "foo", Value: "bar"},
	{Key: "baz", Value: "bif"},
	{Key: "emptyValue", Value: ""},
}

func parityPool(t *testing.T) *apool.APool {
	t.Helper()
	pool := apool.NewAPool()
	for i, a := range parityAttribs {
		require.Equal(t, i, pool.PutAttrib(a, nil))
	}
	return &pool
}

func poolSize(pool *apool.APool) int {
	n := 0
	pool.EachAttrib(func(apool.Attribute) { n++ })
	return n
}

func sortedAttribs(attribs []apool.Attribute) []apool.Attribute {
	out := append([]apool.Attribute{}, attribs...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Key != out[j].Key {
			return out[i].Key < out[j].Key
		}
		return out[i].Value < out[j].Value
	})
	return out
}

func attribString(t *testing.T, attribs []apool.Attribute, pool *apool.APool) string {
	t.Helper()
	s, err := AttribsToString(sortedAttribs(attribs), pool)
	require.NoError(t, err)
	return *s
}

func TestAttributeMapStringSortsAttributes(t *testing.T) {
	pool := parityPool(t)
	m := NewAttributeMap(pool).Update(parityAttribs, nil)
	got := AttribsFromString(m.String(), *pool)
	want := sortedAttribs(parityAttribs)
	// The input must not be sorted already, or this would pass by accident.
	require.NotEqual(t, want, parityAttribs)
	require.Equal(t, want, got)
}

func TestAttributeMapStringReturnsAllEntries(t *testing.T) {
	pool := parityPool(t)
	del := true
	m := NewAttributeMap(pool)
	require.Equal(t, "", m.String())
	m.Set(parityAttribs[0].Key, parityAttribs[0].Value)
	require.Equal(t, "*0", m.String())
	// Removing an entry: an empty value with emptyValueIsDelete.
	m.Update([]apool.Attribute{{Key: parityAttribs[0].Key, Value: ""}}, &del)
	require.Equal(t, "", m.String())
	m.Set(parityAttribs[1].Key, parityAttribs[1].Value)
	require.Equal(t, "*1", m.String())
	// A new value is a new pool entry, after the three existing ones.
	m.Set(parityAttribs[1].Key, "new value")
	require.Equal(t, "*3", m.String())
	m.Set(parityAttribs[2].Key, parityAttribs[2].Value)
	require.Equal(t, attribString(t, []apool.Attribute{parityAttribs[2], {Key: parityAttribs[1].Key, Value: "new value"}}, pool), m.String())
}

func TestAttributeMapUpdate(t *testing.T) {
	// The same cases for both ways to update a map.
	updaters := map[string]func(t *testing.T, m *AttributeMap, pool *apool.APool, attribs []apool.Attribute) *AttributeMap{
		"Update": func(t *testing.T, m *AttributeMap, pool *apool.APool, attribs []apool.Attribute) *AttributeMap {
			return m.Update(attribs, nil)
		},
		"UpdateFromString": func(t *testing.T, m *AttributeMap, pool *apool.APool, attribs []apool.Attribute) *AttributeMap {
			return m.UpdateFromString(attribString(t, attribs, pool), nil)
		},
	}
	for name, update := range updaters {
		t.Run(name, func(t *testing.T) {
			t.Run("works", func(t *testing.T) {
				pool := parityPool(t)
				m := NewAttributeMap(pool)
				m.Set(parityAttribs[2].Key, "value to be overwritten")
				update(t, m, pool, parityAttribs)
				for _, a := range parityAttribs {
					got := m.Get(a.Key)
					require.NotNil(t, got, a.Key)
					require.Equal(t, a.Value, *got)
				}
				require.Equal(t, len(parityAttribs), m.Size())
				want := attribString(t, parityAttribs, pool)
				require.Equal(t, want, m.String())
				update(t, m, pool, nil)
				require.Equal(t, want, m.String(), "an empty update changes nothing")
			})

			t.Run("inserts new attributes into the pool", func(t *testing.T) {
				pool := parityPool(t)
				m := NewAttributeMap(pool)
				update(t, m, pool, []apool.Attribute{{Key: "k", Value: "v"}})
				require.Equal(t, 1, m.Size())
				require.Equal(t, "v", *m.Get("k"))
				require.Equal(t, len(parityAttribs)+1, poolSize(pool))
				require.Equal(t, "*3", m.String())
			})

			t.Run("returns the map", func(t *testing.T) {
				pool := parityPool(t)
				m := NewAttributeMap(pool)
				require.Same(t, m, update(t, m, pool, nil))
			})
		})
	}
}

func TestAttributeMapSetReturnsTheMap(t *testing.T) {
	m := NewAttributeMap(parityPool(t))
	require.Same(t, m, m.Set("k", "v"))
}
