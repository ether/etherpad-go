package pad

import (
	"errors"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/models/pad"
	"regexp"
)

var globalPadCache *GlobalPadCache

func init() {
	globalPadCache = &GlobalPadCache{
		padCache: make(map[string]*pad.Pad),
	}
}

type GlobalPadCache struct {
	padCache map[string]*pad.Pad
}

func (g *GlobalPadCache) GetPad(padID string) *pad.Pad {
	return g.padCache[padID]
}

func (g *GlobalPadCache) SetPad(padID string, pad *pad.Pad) {
	g.padCache[padID] = pad
}

func (g *GlobalPadCache) DeletePad(padID string) {
	delete(g.padCache, padID)
}

var padRegex *regexp.Regexp

func init() {
	padRegex, _ = regexp.Compile("^(g.[a-zA-Z0-9]{16}$)?[^$]{1,50}$")
}

type Manager struct {
	store db.DataStore
}

func (m *Manager) doesPadExist(padID string) bool {
	return m.store.DoesPadExist(padID)
}

func (m *Manager) createPad(padID string) bool {
	if !m.store.CreatePad(padID) {

	}
	return true
}

func (m *Manager) isValidPadId(padID string) bool {
	return padRegex.MatchString(padID)
}

func (m *Manager) SanitizePadId(padID string) string {
	if m.isValidPadId(padID) {
		return padID
	}
	return padID
}

func (m *Manager) GetPad(padID string, text *string, author *author.Author) (*pad.Pad, error) {
	if m.isValidPadId(padID) {
		return nil, errors.New("Invalid pad id")
	}

	if text != nil {
		if len(*text) > 100000 {
			return nil, errors.New("Text is too long")
		}
	}

	var cachedPad = globalPadCache.GetPad(padID)

	if cachedPad != nil {
		return cachedPad, nil
	}

	// try to load pad
	var newPad = pad.NewPad(padID)

	// initialize the pad
	newPad.Init(text, author.Id)
}
