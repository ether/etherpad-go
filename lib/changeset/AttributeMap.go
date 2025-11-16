package changeset

import (
	"slices"
	"strings"

	"github.com/ether/etherpad-go/lib/apool"
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

func FromString(s string, pool *apool.APool) AttributeMap {
	var mapInAttr = NewAttributeMap(pool)
	mapInAttr.UpdateFromString(s, nil)

	return mapInAttr
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

func (a *AttributeMap) Has(key string) bool {
	_, ok := a.attrs[key]
	return ok
}

func (a *AttributeMap) Size() int {
	return len(a.attrs)
}

func (a *AttributeMap) Set(key string, value string) *AttributeMap {
	a.attrs[key] = value
	a.pool.PutAttrib(apool.Attribute{Key: key, Value: value}, nil)
	return a
}

func (a *AttributeMap) Get(key string) *string {
	val, ok := a.attrs[key]
	if !ok {
		return nil
	}
	return &val
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

	var attribs = AttribsFromString(key, *a.pool)
	return a.Update(attribs, &localEmptyValueIsDelete)
}

func (a *AttributeMap) String() string {
	resolvedString, err := AttribsToString(a.sortAttribs(), a.pool)
	if err != nil {
		return ""
	}
	return *resolvedString
}

func (a *AttributeMap) sortAttribs() []apool.Attribute {
	var copiedSlice = make([]apool.Attribute, 0)
	for key, value := range a.attrs {
		copiedSlice = append(copiedSlice, apool.Attribute{Key: key, Value: value})
	}
	slices.SortFunc(copiedSlice, apool.CmpAttribute)
	return copiedSlice
}
