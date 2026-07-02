package sheet

import (
	"maps"
	"sort"
	"strings"
)

// Style is a set of formatting properties (e.g. numFmt, bold, color, align,
// border). Kept as a string->string map so the pool stays format-agnostic.
type Style struct {
	Props map[string]string `json:"props"`
}

// canonicalKey produces a deterministic key independent of map iteration order,
// so equal styles dedup regardless of insertion order.
func (s Style) canonicalKey() string {
	if len(s.Props) == 0 {
		return ""
	}
	keys := make([]string, 0, len(s.Props))
	for k := range s.Props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		b.WriteString(k)
		b.WriteByte('\x00')
		b.WriteString(s.Props[k])
		b.WriteByte('\x01')
	}
	return b.String()
}

// StylePool deduplicates styles per workbook. Id 0 is reserved for the empty
// style; cells default to it.
type StylePool struct {
	IdToStyle map[int]Style `json:"idToStyle"`
	keyToId   map[string]int
	NextId    int `json:"nextId"`
}

func NewStylePool() *StylePool {
	return &StylePool{
		IdToStyle: map[int]Style{},
		keyToId:   map[string]int{"": 0},
		NextId:    1,
	}
}

// Put interns a style and returns its id (dedup by canonical key).
func (p *StylePool) Put(s Style) int {
	key := s.canonicalKey()
	if id, ok := p.keyToId[key]; ok {
		return id
	}
	id := p.NextId
	p.NextId++
	// Copy the props map: callers (e.g. Apply) pass maps they own, and a later
	// mutation of an aliased map would desync keyToId from the stored content.
	p.IdToStyle[id] = Style{Props: maps.Clone(s.Props)}
	p.keyToId[key] = id
	return id
}

// Get returns the style for an id. Id 0 is always the empty style.
func (p *StylePool) Get(id int) (Style, bool) {
	if id == 0 {
		return Style{}, true
	}
	s, ok := p.IdToStyle[id]
	return s, ok
}

// rebuildIndex repopulates keyToId after deserialization (json only restores
// the exported maps). Call after unmarshaling a pool.
func (p *StylePool) rebuildIndex() {
	p.keyToId = map[string]int{"": 0}
	for id, s := range p.IdToStyle {
		p.keyToId[s.canonicalKey()] = id
	}
}

// clone returns a deep copy of the pool.
func (p *StylePool) clone() *StylePool {
	cp := &StylePool{IdToStyle: make(map[int]Style, len(p.IdToStyle)), NextId: p.NextId}
	for id, s := range p.IdToStyle {
		cp.IdToStyle[id] = Style{Props: maps.Clone(s.Props)}
	}
	cp.rebuildIndex()
	return cp
}
