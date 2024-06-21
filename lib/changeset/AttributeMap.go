package changeset

import (
	"github.com/ether/etherpad-go/lib/apool"
	"strings"
)

type AttributeMap struct {
	pool  *apool.APool
	attrs map[string]string
}

func NewAttributeMap(pool *apool.APool) AttributeMap {
	return AttributeMap{
		pool:  pool,
		attrs: make(map[string]string),
	}
}

func FromString(s string, pool apool.APool) AttributeMap {
	var AttrMap = AttributeMap{
		pool:  &pool,
		attrs: make(map[string]string),
	}
	AttrMap.UpdateFromString(s, nil)

	return AttrMap
}

/**
 * @param {Iterable<Attribute>} entries - [key, value] pairs to insert into this map.
 * @param {boolean} [emptyValueIsDelete] - If true and an entry's value is the empty string, the
 *     key is removed from this map (if present).
 * @returns {AttributeMap} `this` (for chaining).
 */
func (a *AttributeMap) Update(entries []apool.Attribute, emptyValueISDelete *bool) *AttributeMap {
	if emptyValueISDelete == nil {
		*emptyValueISDelete = false
	}

	for _, entry := range entries {
		entry.Key = strings.TrimSpace(entry.Key)
		entry.Value = strings.TrimSpace(entry.Value)

		if entry.Value == "" && *emptyValueISDelete {
			delete(a.attrs, entry.Key)
		} else {
			a.attrs[entry.Key] = entry.Value
		}
	}
	return a
}

func (a *AttributeMap) ToArray() []apool.Attribute {
	var attribs = make([]apool.Attribute, 0)
	for key, value := range a.attrs {
		attribs = append(attribs, apool.Attribute{Key: key, Value: value})
	}
	return attribs
}

/**
 * @param {AttributeString} str - The attribute string identifying the attributes to insert into
 *     this map.
 * @param {boolean} [emptyValueIsDelete] - If true and an entry's value is the empty string, the
 *     key is removed from this map (if present).
 * @returns {AttributeMap} `this` (for chaining).
 */
func (a *AttributeMap) UpdateFromString(key string, emptyValueIsDelete *bool) *AttributeMap {
	var localEmptyValueIsDelete bool
	if emptyValueIsDelete == nil {
		localEmptyValueIsDelete = false
	} else {
		localEmptyValueIsDelete = *emptyValueIsDelete
	}
	var attribs = AttribsFromString(key, a.pool)
	return a.Update(attribs, &localEmptyValueIsDelete)
}

func (a *AttributeMap) String() string {
	return AttribsToString(a.ToArray(), a.pool)
}
