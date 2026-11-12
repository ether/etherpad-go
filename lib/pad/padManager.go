package pad

import (
	"errors"
	"fmt"
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
	padLocks       *padLocks
	// loadMu makes load-or-create of an uncached pad atomic, so concurrent
	// callers can't each build their own *Pad and overwrite one another in
	// the cache (a writer could then keep appending to an orphaned object).
	loadMu sync.Mutex
	// dbLocked counts, per pad, the LockPad calls holding the pad's
	// database lock, so loading the pad under such a call does not try to
	// take that lock a second time.
	dbLockedMu sync.Mutex
	dbLocked   map[string]int
}

func (m *Manager) markDBLocked(ids []string, delta int) {
	m.dbLockedMu.Lock()
	defer m.dbLockedMu.Unlock()
	if m.dbLocked == nil {
		m.dbLocked = map[string]int{}
	}
	for _, id := range ids {
		if m.dbLocked[id] += delta; m.dbLocked[id] <= 0 {
			delete(m.dbLocked, id)
		}
	}
}

func (m *Manager) isDBLocked(id string) bool {
	m.dbLockedMu.Lock()
	defer m.dbLockedMu.Unlock()
	return m.dbLocked[id] > 0
}

// lockForLoad takes the database lock of an uncached pad for loading or
// creating it, so instances creating the same pad at once cannot see each
// other's half-created pad (a row whose first revision is not stored yet).
// It is skipped when a LockPad call already holds the lock. It must be taken
// before loadMu: LockPad holders take loadMu while holding the lock.
func (m *Manager) lockForLoad(padID string) (func(), error) {
	if m.isDBLocked(padID) {
		return func() {}, nil
	}
	release, err := m.store.LockPadsExclusive([]string{padID})
	if err != nil {
		return nil, fmt.Errorf("%w: locking %s for loading: %v", ErrPadUnavailable, padID, err)
	}
	return release, nil
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
		padList:  NewList(db),
		padLocks: newPadLocks(),
	}
}

func (m *Manager) DoesPadExist(padID string) (*bool, error) {
	return m.store.DoesPadExist(padID)
}

func (m *Manager) IsValidPadId(padID string) bool {
	return pad.IsValidPadId(padID)
}

// padIdTransforms are applied one after another by SanitizePadId until a pad
// with the resulting id exists, so ids of older formats are still found.
var padIdTransforms = []struct {
	from *regexp.Regexp
	to   string
}{
	{regexp.MustCompile(`\s+`), "_"},
	{regexp.MustCompile(`:+`), "_"},
}

// SanitizePadId maps a requested pad id to the one to use, like
// etherpad-lite's sanitizePadId: an existing pad keeps its id; otherwise
// whitespace and then ":" are replaced by "_", stopping as soon as a pad with
// the id exists. The result may still be invalid ("a$b"); callers check it
// with CanOpenPadId.
func (m *Manager) SanitizePadId(padID string) (string, error) {
	for _, t := range padIdTransforms {
		exists, err := m.DoesPadExist(padID)
		if err != nil {
			return "", err
		}
		if *exists {
			return padID, nil
		}
		padID = t.from.ReplaceAllString(padID, t.to)
	}
	return padID, nil
}

// CanOpenPadId reports whether a pad with this id may be opened: either the id
// is valid for a new pad, or a pad with exactly this id already exists (ids
// with ":" were allowed before, see pad.IsLegacyPadId).
func (m *Manager) CanOpenPadId(padID string) (bool, error) {
	if m.IsValidPadId(padID) {
		return true, nil
	}
	if !pad.IsLegacyPadId(padID) {
		return false, nil
	}
	exists, err := m.DoesPadExist(padID)
	if err != nil {
		return false, err
	}
	return *exists, nil
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
	if ok, err := m.CanOpenPadId(padID); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("invalid pad id")
	}

	if text != nil {
		if utf8.RuneCountInString(*text) > 100000 {
			return nil, errors.New("text is too long")
		}
	}

	if cachedPad := m.globalPadCache.GetPad(padID); cachedPad != nil {
		return cachedPad, nil
	}

	release, err := m.lockForLoad(padID)
	if err != nil {
		return nil, err
	}
	defer release()
	m.loadMu.Lock()
	defer m.loadMu.Unlock()
	// Another caller may have loaded it while we waited.
	if cachedPad := m.globalPadCache.GetPad(padID); cachedPad != nil {
		return cachedPad, nil
	}

	// try to load pad
	var newPad = pad.NewPad(padID, m.store, m.hook)

	// initialize the pad; never cache a pad that failed to load
	if err := newPad.Init(text, authorId, m.author); err != nil {
		return nil, err
	}
	m.globalPadCache.SetPad(padID, &newPad)

	return &newPad, nil
}

// GetTypedPad loads or creates a pad of a specific document type. For an
// existing pad the stored type is kept; documentType only applies on first
// creation (it is set before Init so Init -> AppendRevision -> Save persists
// it via CreatePad).
func (m *Manager) GetTypedPad(padID string, documentType string, authorId *string) (*pad.Pad, error) {
	if ok, err := m.CanOpenPadId(padID); err != nil {
		return nil, err
	} else if !ok {
		return nil, errors.New("invalid pad id")
	}

	if cachedPad := m.globalPadCache.GetPad(padID); cachedPad != nil {
		return cachedPad, nil
	}

	release, err := m.lockForLoad(padID)
	if err != nil {
		return nil, err
	}
	defer release()
	m.loadMu.Lock()
	defer m.loadMu.Unlock()
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
