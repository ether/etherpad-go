package pad

import (
	"errors"
	"regexp"
	"sync"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/models/pad"
)

type List struct {
	_cachedList []string
	_list       map[string]interface{}
	_loaded     bool
	db          db.DataStore
}

func NewList(db db.DataStore) List {
	return List{
		_cachedList: make([]string, 0),
		_list:       make(map[string]interface{}),
		_loaded:     false,
		db:          db,
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
		var dbData, err = l.db.GetPadIds()
		if err != nil {
			return l._cachedList
		}
		for _, padId := range *dbData {
			l.AddPad(padId)
		}
	}
	return l._cachedList
}

var padRegex *regexp.Regexp

func init() {
	padRegex, _ = regexp.Compile(`^(g\.[A-Za-z0-9]{16}\$)?[^ \t\r\n\f\v$]{1,50}$`)
}

type GlobalPadCache struct {
	padCache map[string]*pad.Pad
	mutex    sync.RWMutex
}

func (g *GlobalPadCache) GetPad(padID string) *pad.Pad {
	g.mutex.RLock()
	defer g.mutex.RUnlock()
	return g.padCache[padID]
}

func (g *GlobalPadCache) SetPad(padID string, pad *pad.Pad) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.padCache[padID] = pad
}

func (g *GlobalPadCache) DeletePad(padID string) {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	delete(g.padCache, padID)
}

type Manager struct {
	store          db.DataStore
	globalPadCache *GlobalPadCache
	author         *author.Manager
	hook           *hooks.Hook
	padList        List
}

func NewManager(db db.DataStore, hook *hooks.Hook) *Manager {
	return &Manager{
		store: db,
		hook:  hook,
		author: &author.Manager{
			Db: db,
		},
		globalPadCache: &GlobalPadCache{
			padCache: make(map[string]*pad.Pad),
			mutex:    sync.RWMutex{},
		},
		padList: NewList(db),
	}
}

func (m *Manager) DoesPadExist(padID string) (*bool, error) {
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
	// Capture the loaded pad (if any) before deletion so the padRemove hook can
	// hand listeners the pad context, mirroring the original Etherpad which
	// fires padRemove from Pad.remove() with `this`.
	removedPad := m.globalPadCache.GetPad(padID)

	if err := m.store.RemovePad(padID); err != nil {
		return err
	}
	m.globalPadCache.DeletePad(padID)
	m.padList.RemovePad(padID)

	m.hook.ExecutePadRemoveHooks(&events.PadRemoveContext{
		Pad:   removedPad,
		PadId: padID,
	})

	return nil
}

func (m *Manager) GetPad(padID string, text *string, authorId *string) (*pad.Pad, error) {
	if !m.IsValidPadId(padID) {
		return nil, errors.New("invalid pad id")
	}

	if text != nil {
		if utf8.RuneCountInString(*text) > 100000 {
			return nil, errors.New("text is too long")
		}
	}

	var cachedPad = m.globalPadCache.GetPad(padID)

	if cachedPad != nil {
		return cachedPad, nil
	}

	// try to load pad
	var newPad = pad.NewPad(padID, m.store, m.hook)

	// initialize the pad

	newPad.Init(text, authorId, m.author)
	m.globalPadCache.SetPad(padID, &newPad)

	return &newPad, nil
}

// GetTypedPad loads or creates a pad of a specific document type. For an
// existing pad the stored type is kept; documentType only applies on first
// creation (it is set before Init so Init -> AppendRevision -> Save persists
// it via CreatePad).
func (m *Manager) GetTypedPad(padID string, documentType string, authorId *string) (*pad.Pad, error) {
	if !m.IsValidPadId(padID) {
		return nil, errors.New("invalid pad id")
	}

	if cachedPad := m.globalPadCache.GetPad(padID); cachedPad != nil {
		return cachedPad, nil
	}

	newPad := pad.NewPad(padID, m.store, m.hook)
	newPad.DocumentType = documentType

	if err := newPad.Init(nil, authorId, m.author); err != nil {
		return nil, err
	}
	m.globalPadCache.SetPad(padID, &newPad)
	return &newPad, nil
}

func (m *Manager) UnloadPad(id string) {
	m.globalPadCache.DeletePad(id)
	m.padList.RemovePad(id)
}
