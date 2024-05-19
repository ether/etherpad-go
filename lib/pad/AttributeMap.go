package pad

import (
	"github.com/ether/etherpad-go/lib/models/pad"
	"strings"
)

type AttributeMap struct {
	pool  pad.APool
	attrs map[string]string
}

func FromString(s string, pool pad.APool) AttributeMap {
	var AttrMap = AttributeMap{
		pool:  pool,
		attrs: make(map[string]string),
	}
	AttrMap.UpdateFromString(s)

	return AttrMap
}

/**
 * @param {Iterable<Attribute>} entries - [key, value] pairs to insert into this map.
 * @param {boolean} [emptyValueIsDelete] - If true and an entry's value is the empty string, the
 *     key is removed from this map (if present).
 * @returns {AttributeMap} `this` (for chaining).
 */
func (a *AttributeMap) Update(entries []pad.Attribute, emptyValueISDelete *bool) *AttributeMap {
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

/**
 * @param {AttributeString} str - The attribute string identifying the attributes to insert into
 *     this map.
 * @param {boolean} [emptyValueIsDelete] - If true and an entry's value is the empty string, the
 *     key is removed from this map (if present).
 * @returns {AttributeMap} `this` (for chaining).
 */
func (a *AttributeMap) UpdateFromString(key string) AttributeMap {
	return a.Update()
}
