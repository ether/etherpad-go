package db

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
	"github.com/ory/fosite"
)

type MemoryDataStore struct {
	padStore      map[string]db.PadDB
	padRevisions  map[string]map[int]db.PadSingleRevision
	authorStore   map[string]db.AuthorDB
	chatPads      map[string]db.ChatMessageDB
	sessionStore  map[string]session2.Session
	groupStore    map[string]string
	serverVersion string

	// oidc
	accessTokens           map[string]fosite.Requester
	accessTokenRequestIDs  map[string]string
	refreshTokens          map[string]db.StoreRefreshToken
	refreshTokenRequestIDs map[string]string
}

func (m *MemoryDataStore) Ping() error {
	return nil
}

func (m *MemoryDataStore) GetAuthors(ids []string) (*[]db.AuthorDB, error) {
	var authors []db.AuthorDB
	for _, id := range ids {
		author, ok := m.authorStore[id]
		if ok {
			authors = append(authors, author)
		}
		if !ok {
			return nil, errors.New(AuthorNotFoundError)
		}
	}
	return &authors, nil
}

// ============== PAD METHODS ==============

func (m *MemoryDataStore) CreatePad(padID string, padDB db.PadDB) error {
	now := time.Now()

	existing, exists := m.padStore[padID]
	var nowTime = time.Now()
	if exists {
		padDB.CreatedAt = existing.CreatedAt
		padDB.UpdatedAt = &nowTime
	} else {
		// New pad
		padDB.CreatedAt = now
		padDB.UpdatedAt = &nowTime
		padDB.ID = padID
		m.padRevisions[padID] = make(map[int]db.PadSingleRevision)
	}

	m.padStore[padID] = padDB
	return nil
}

func (m *MemoryDataStore) GetPad(padID string) (*db.PadDB, error) {
	pad, ok := m.padStore[padID]
	if !ok {
		return nil, errors.New(PadDoesNotExistError)
	}
	return &pad, nil
}

func (m *MemoryDataStore) DoesPadExist(padID string) (*bool, error) {
	_, ok := m.padStore[padID]
	return &ok, nil
}

func (m *MemoryDataStore) RemovePad(padID string) error {
	delete(m.padStore, padID)
	delete(m.padRevisions, padID)
	return nil
}

func (m *MemoryDataStore) GetPadIds() (*[]string, error) {
	var padIds []string
	for k := range m.padStore {
		padIds = append(padIds, k)
	}
	return &padIds, nil
}

func (m *MemoryDataStore) SaveChatHeadOfPad(padId string, head int) error {
	pad, ok := m.padStore[padId]
	if !ok {
		return errors.New(PadDoesNotExistError)
	}
	nowTime := time.Now()
	pad.ChatHead = head
	pad.UpdatedAt = &nowTime
	m.padStore[padId] = pad
	return nil
}

// ============== READONLY METHODS (simplified) ==============

func (m *MemoryDataStore) GetReadonlyPad(padId string) (*string, error) {
	pad, ok := m.padStore[padId]
	if !ok {
		return nil, errors.New(PadDoesNotExistError)
	}

	if pad.ReadOnlyId == nil {
		return nil, errors.New(PadReadOnlyIdNotFoundError)
	}

	return pad.ReadOnlyId, nil
}

func (m *MemoryDataStore) SetReadOnlyId(padId string, readonlyId string) error {
	pad, ok := m.padStore[padId]
	if !ok {
		return errors.New(PadDoesNotExistError)
	}

	pad.ReadOnlyId = &readonlyId
	UpdatedNow := time.Now()
	pad.UpdatedAt = &UpdatedNow
	m.padStore[padId] = pad
	return nil
}

func (m *MemoryDataStore) GetPadByReadOnlyId(readonlyId string) (*string, error) {
	for padId, pad := range m.padStore {
		if pad.ReadOnlyId != nil && *pad.ReadOnlyId == readonlyId {
			return &padId, nil
		}
	}
	return nil, nil
}

// Deprecated: Use SetReadOnlyId instead
func (m *MemoryDataStore) CreatePad2ReadOnly(padId string, readonlyId string) error {
	return m.SetReadOnlyId(padId, readonlyId)
}

// Deprecated: Use SetReadOnlyId instead - readonly2pad is derived from pad store
func (m *MemoryDataStore) CreateReadOnly2Pad(padId string, readonlyId string) error {
	return nil // No-op, data is in pad store
}

// Deprecated: Handled by RemovePad
func (m *MemoryDataStore) RemovePad2ReadOnly(id string) error {
	return nil // No-op, readonly_id is deleted with pad
}

// Deprecated: Handled by RemovePad
func (m *MemoryDataStore) RemoveReadOnly2Pad(id string) error {
	return nil // No-op
}

// Deprecated: Use GetPadByReadOnlyId instead
func (m *MemoryDataStore) GetReadOnly2Pad(id string) (*string, error) {
	return m.GetPadByReadOnlyId(id)
}

// ============== AUTHOR METHODS ==============

func (m *MemoryDataStore) SaveAuthor(author db.AuthorDB) error {
	if author.ID == "" {
		return errors.New("author ID is empty")
	}

	now := time.Now()
	existing, exists := m.authorStore[author.ID]

	if exists {
		// Preserve token if not provided
		if author.Token == nil && existing.Token != nil {
			author.Token = existing.Token
		}
		author.CreatedAt = existing.CreatedAt
	} else {
		author.CreatedAt = now
	}

	m.authorStore[author.ID] = author
	return nil
}

func (m *MemoryDataStore) GetPadIdsOfAuthor(authorId string) (*[]string, error) {
	padIDSet := make(map[string]struct{})
	for padId, revisions := range m.padRevisions {
		for _, rev := range revisions {
			if rev.AuthorId != nil && *rev.AuthorId == authorId {
				padIDSet[padId] = struct{}{}
			}
		}
	}
	var padIDs []string
	for padID := range padIDSet {
		padIDs = append(padIDs, padID)
	}
	return &padIDs, nil
}

func (m *MemoryDataStore) GetAuthor(authorId string) (*db.AuthorDB, error) {
	retrievedAuthor, ok := m.authorStore[authorId]
	if !ok {
		return nil, errors.New(AuthorNotFoundError)
	}

	// Build PadIDs from revisions
	padIDs := make(map[string]struct{})
	for padId, revisions := range m.padRevisions {
		for _, rev := range revisions {
			if rev.AuthorId != nil && *rev.AuthorId == authorId {
				padIDs[padId] = struct{}{}
				break
			}
		}
	}
	return &retrievedAuthor, nil
}

func (m *MemoryDataStore) SetAuthorByToken(token string, authorId string) error {
	// Check if author exists
	author, exists := m.authorStore[authorId]
	if exists {
		author.Token = &token
		m.authorStore[authorId] = author
		return nil
	}

	// Create new author with token
	m.authorStore[authorId] = db.AuthorDB{
		ID:        authorId,
		Token:     &token,
		ColorId:   "",
		Timestamp: 0,
		CreatedAt: time.Now(),
	}
	return nil
}

func (m *MemoryDataStore) GetAuthorByToken(token string) (*string, error) {
	for id, author := range m.authorStore {
		if author.Token != nil && *author.Token == token {
			return &id, nil
		}
	}
	return nil, errors.New(AuthorNotFoundError)
}

func (m *MemoryDataStore) SaveAuthorName(authorId string, authorName string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}

	retrievedAuthor, ok := m.authorStore[authorId]
	if !ok {
		return errors.New("author not found")
	}

	retrievedAuthor.Name = &authorName
	m.authorStore[authorId] = retrievedAuthor
	return nil
}

func (m *MemoryDataStore) SaveAuthorColor(authorId string, authorColor string) error {
	if authorId == "" {
		return errors.New("authorId is empty")
	}

	retrievedAuthor, ok := m.authorStore[authorId]
	if !ok {
		return errors.New("author not found")
	}

	retrievedAuthor.ColorId = authorColor
	m.authorStore[authorId] = retrievedAuthor
	return nil
}

// ============== REVISION METHODS ==============

func (m *MemoryDataStore) SaveRevision(
	padId string,
	rev int,
	changeset string,
	text db.AText,
	pool db.RevPool,
	authorId *string,
	timestamp int64,
) error {
	_, ok := m.padStore[padId]
	if !ok {
		return errors.New(PadDoesNotExistError)
	}

	if m.padRevisions[padId] == nil {
		m.padRevisions[padId] = make(map[int]db.PadSingleRevision)
	}

	// Write-once: don't overwrite existing revision
	if _, exists := m.padRevisions[padId][rev]; exists {
		return nil
	}

	m.padRevisions[padId][rev] = db.PadSingleRevision{
		PadId:     padId,
		RevNum:    rev,
		Changeset: changeset,
		AText:     text,
		AuthorId:  authorId,
		Timestamp: timestamp,
		Pool:      &pool,
	}

	return nil
}

func (m *MemoryDataStore) GetRevision(padId string, rev int) (*db.PadSingleRevision, error) {
	_, ok := m.padStore[padId]
	if !ok {
		return nil, errors.New(PadDoesNotExistError)
	}

	if m.padRevisions[padId] == nil {
		return nil, errors.New(PadRevisionNotFoundError)
	}

	revisionFromPad, ok := m.padRevisions[padId][rev]
	if !ok {
		return nil, errors.New(PadRevisionNotFoundError)
	}

	return &db.PadSingleRevision{
		PadId:     padId,
		RevNum:    rev,
		Pool:      revisionFromPad.Pool,
		Changeset: revisionFromPad.Changeset,
		AText:     revisionFromPad.AText,
		AuthorId:  revisionFromPad.AuthorId,
		Timestamp: revisionFromPad.Timestamp,
	}, nil
}

func (m *MemoryDataStore) GetRevisions(padId string, startRev int, endRev int) (*[]db.PadSingleRevision, error) {
	_, ok := m.padStore[padId]
	if !ok {
		return nil, errors.New(PadDoesNotExistError)
	}

	revisions, ok := m.padRevisions[padId]
	revisionsToReturn := make([]db.PadSingleRevision, 0)

	if !ok {
		return nil, errors.New(PadRevisionNotFoundError)
	}

	for rev := startRev; rev <= endRev; rev++ {
		revisionFromPad, ok := revisions[rev]
		if !ok {
			return nil, errors.New(PadRevisionNotFoundError)
		}

		revisionsToReturn = append(revisionsToReturn, db.PadSingleRevision{
			PadId:     padId,
			RevNum:    rev,
			Pool:      revisionFromPad.Pool,
			Changeset: revisionFromPad.Changeset,
			AText:     revisionFromPad.AText,
			AuthorId:  revisionFromPad.AuthorId,
			Timestamp: revisionFromPad.Timestamp,
		})
	}

	if len(revisionsToReturn) != (endRev - startRev + 1) {
		return nil, errors.New(PadRevisionNotFoundError)
	}

	return &revisionsToReturn, nil
}

func (m *MemoryDataStore) RemoveRevisionsOfPad(padId string) error {
	_, ok := m.padStore[padId]
	if !ok {
		return errors.New(PadDoesNotExistError)
	}

	m.padRevisions[padId] = make(map[int]db.PadSingleRevision)
	return nil
}

// ============== CHAT METHODS ==============

func calcChatMessageKey(padId string, head int) string {
	return padId + ":" + strconv.Itoa(head)
}

func (m *MemoryDataStore) SaveChatMessage(
	padId string,
	head int,
	authorId *string,
	timestamp int64,
	text string,
) error {
	key := calcChatMessageKey(padId, head)

	// Write-once: don't overwrite existing message
	if _, exists := m.chatPads[key]; exists {
		return nil
	}

	m.chatPads[key] = db.ChatMessageDB{
		PadId:    padId,
		Head:     head,
		AuthorId: authorId,
		Time:     &timestamp,
		Message:  text,
	}
	return nil
}

func (m *MemoryDataStore) GetChatsOfPad(
	padId string,
	start int,
	end int,
) (*[]db.ChatMessageDBWithDisplayName, error) {
	var chatMessages []db.ChatMessageDBWithDisplayName

	for head := start; head <= end; head++ {
		chatMessageKey := calcChatMessageKey(padId, head)
		chatMessage, ok := m.chatPads[chatMessageKey]
		if ok {
			var displayName *string
			if chatMessage.AuthorId != nil {
				if authorFromDB, ok := m.authorStore[*chatMessage.AuthorId]; ok {
					displayName = authorFromDB.Name
				}
			}
			chatMessages = append(chatMessages, db.ChatMessageDBWithDisplayName{
				ChatMessageDB: chatMessage,
				DisplayName:   displayName,
			})
		}
	}

	return &chatMessages, nil
}

func (m *MemoryDataStore) GetAuthorIdsOfPadChats(id string) (*[]string, error) {
	authorIdSet := make(map[string]struct{})

	for k, chatMessage := range m.chatPads {
		if strings.HasPrefix(k, id+":") {
			if chatMessage.AuthorId != nil {
				authorIdSet[*chatMessage.AuthorId] = struct{}{}
			}
		}
	}

	var authorIds []string
	for authorId := range authorIdSet {
		authorIds = append(authorIds, authorId)
	}

	return &authorIds, nil
}

func (m *MemoryDataStore) RemoveChat(padId string) error {
	for k := range m.chatPads {
		if strings.HasPrefix(k, padId+":") {
			delete(m.chatPads, k)
		}
	}
	return nil
}

// ============== GROUP METHODS ==============

func (m *MemoryDataStore) SaveGroup(groupId string) error {
	m.groupStore[groupId] = groupId
	return nil
}

func (m *MemoryDataStore) RemoveGroup(groupId string) error {
	delete(m.groupStore, groupId)
	return nil
}

func (m *MemoryDataStore) GetGroup(groupId string) (*string, error) {
	group, ok := m.groupStore[groupId]
	if !ok {
		return nil, errors.New("group not found")
	}
	return &group, nil
}

// ============== SESSION METHODS ==============

func (m *MemoryDataStore) GetSessionById(sessionID string) (*session2.Session, error) {
	retrievedSession, ok := m.sessionStore[sessionID]
	if !ok {
		return nil, nil
	}
	return &retrievedSession, nil
}

func (m *MemoryDataStore) SetSessionById(sessionID string, session session2.Session) error {
	m.sessionStore[sessionID] = session
	return nil
}

func (m *MemoryDataStore) RemoveSessionById(sessionID string) error {
	_, ok := m.sessionStore[sessionID]
	if !ok {
		return errors.New(SessionNotFoundError)
	}
	delete(m.sessionStore, sessionID)
	return nil
}

// ============== QUERY/SEARCH METHODS ==============

func (m *MemoryDataStore) QueryPad(
	offset int,
	limit int,
	sortBy string,
	ascending bool,
	pattern string,
) (*db.PadDBSearchResult, error) {
	var padKeys []string
	for k := range m.padStore {
		padKeys = append(padKeys, k)
	}

	// Filter by pattern
	if pattern != "" {
		var filteredPadKeys []string
		for _, key := range padKeys {
			if strings.Contains(key, pattern) {
				filteredPadKeys = append(filteredPadKeys, key)
			}
		}
		padKeys = filteredPadKeys
	}

	// Sort
	if sortBy == "padName" {
		slices.Sort(padKeys)
	} else if sortBy == "lastEdited" {
		slices.SortFunc(padKeys, func(a, b string) int {
			padA := m.padStore[a]
			padB := m.padStore[b]
			if padA.UpdatedAt == nil && padB.UpdatedAt == nil {
				return 0
			}
			if padA.UpdatedAt == nil {
				return -1
			}
			if padB.UpdatedAt == nil {
				return 1
			}
			if padA.UpdatedAt.Before(*padB.UpdatedAt) {
				return -1
			}
			if padA.UpdatedAt.After(*padB.UpdatedAt) {
				return 1
			}
			return 0
		})
	}

	if !ascending {
		slices.Reverse(padKeys)
	}

	// Paginate
	padEnd := int(math.Min(float64(len(padKeys)), float64(offset+limit)))
	padStart := int(math.Max(0, float64(offset)))
	padsToSearch := padKeys[padStart:padEnd]

	padSearch := make([]db.PadDBSearch, 0, len(padsToSearch))
	for _, padKey := range padsToSearch {
		retrievedPad := m.padStore[padKey]
		padSearch = append(padSearch, db.PadDBSearch{
			Padname:        padKey,
			RevisionNumber: retrievedPad.Head,
			LastEdited:     retrievedPad.UpdatedAt.UnixMilli(),
		})
	}

	return &db.PadDBSearchResult{
		TotalPads: len(padKeys),
		Pads:      padSearch,
	}, nil
}

func (m *MemoryDataStore) GetServerVersion() (string, error) {
	return m.serverVersion, nil
}

func (m *MemoryDataStore) SaveServerVersion(version string) error {
	m.serverVersion = version
	return nil
}

// ============== OIDC METHODS ==============

func (m *MemoryDataStore) GetAccessTokenRequestID(requestID string) (*string, error) {
	token, ok := m.accessTokenRequestIDs[requestID]
	if !ok {
		return nil, errors.New("access token request ID not found")
	}
	return &token, nil
}

func (m *MemoryDataStore) SaveAccessTokenRequestID(requestID string, token string) error {
	m.accessTokenRequestIDs[requestID] = token
	return nil
}

// ============== LIFECYCLE ==============

func (m *MemoryDataStore) Close() error {
	return nil
}

func NewMemoryDataStore() *MemoryDataStore {
	return &MemoryDataStore{
		padStore:               make(map[string]db.PadDB),
		padRevisions:           make(map[string]map[int]db.PadSingleRevision),
		authorStore:            make(map[string]db.AuthorDB),
		chatPads:               make(map[string]db.ChatMessageDB),
		sessionStore:           make(map[string]session2.Session),
		groupStore:             make(map[string]string),
		accessTokens:           make(map[string]fosite.Requester),
		accessTokenRequestIDs:  make(map[string]string),
		refreshTokens:          make(map[string]db.StoreRefreshToken),
		refreshTokenRequestIDs: make(map[string]string),
	}
}

var _ DataStore = (*MemoryDataStore)(nil)
