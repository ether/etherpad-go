package pad

import (
	"errors"
	"regexp"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/utils"
)

var globalPadCache *GlobalPadCache
var padList List

type List struct {
	_cachedList []string
	_list       map[string]interface{}
	_loaded     bool
	db          db.DataStore
}

func NewList() List {
	return List{
		_cachedList: make([]string, 0),
		_list:       make(map[string]interface{}),
		_loaded:     false,
		db:          utils.GetDB(),
	}
}

func (l *List) AddPad(padID string) {
	if l._list[padID] == nil {
		l._list[padID] = struct{}{}
		l._cachedList = append(l._cachedList, padID)
	}
}

func (l *List) RemovePad(padID string) {
	if l._list[padID] != nil {
		delete(l._list, padID)
		for i, v := range l._cachedList {
			if v == padID {
				l._cachedList = append(l._cachedList[:i], l._cachedList[i+1:]...)
				break
			}
		}
	}
}

func (l *List) GetPads() []string {
	if !l._loaded {
		var dbData = l.db.GetPadIds()
		for _, padId := range dbData {
			l.AddPad(padId)
		}
	}
	return l._cachedList
}

func init() {
	globalPadCache = &GlobalPadCache{
		padCache: make(map[string]*pad.Pad),
	}
	padList = NewList()
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

func NewManager() Manager {
	return Manager{
		store: utils.GetDB(),
	}
}

func (m *Manager) DoesPadExist(padID string) bool {
	return m.store.DoesPadExist(padID)
}

func (m *Manager) IsValidPadId(padID string) bool {
	return padRegex.MatchString(padID)
}

func (m *Manager) SanitizePadId(padID string) (*string, error) {
	if m.IsValidPadId(padID) {
		return &padID, nil
	}
	return nil, errors.New("invalid pad id")
}

func (m *Manager) RemovePad(padID string) error {
	if err := m.store.RemovePad(padID); err != nil {
		return err
	}
	globalPadCache.DeletePad(padID)
	padList.RemovePad(padID)

	return nil
}

func (m *Manager) GetPad(padID string, text *string, authorId *string) (*pad.Pad, error) {
	if !m.IsValidPadId(padID) {
		return nil, errors.New("invalid pad id")
	}

	if text != nil {
		if len(*text) > 100000 {
			return nil, errors.New("text is too long")
		}
	}

	var cachedPad = globalPadCache.GetPad(padID)

	if cachedPad != nil {
		return cachedPad, nil
	}

	// try to load pad
	var newPad = pad.NewPad(padID)

	// initialize the pad

	newPad.Init(text, authorId)
	globalPadCache.SetPad(padID, &newPad)

	return &newPad, nil
}
