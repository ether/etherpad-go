package db

import (
	"errors"
	"math"
	"slices"
	"strconv"
	"strings"

	"github.com/ether/etherpad-go/lib/models/db"
	session2 "github.com/ether/etherpad-go/lib/models/session"
	"github.com/ory/fosite"
)

type MemoryDataStore struct {
	padStore     map[string]db.PadDB
	padRevisions map[string]map[int]db.PadSingleRevision
	authorStore  map[string]db.AuthorDB
	readonly2Pad map[string]string
	pad2Readonly map[string]string
	chatPads     map[string]db.ChatMessageDB
	sessionStore map[string]session2.Session
	tokenStore   map[string]string
	groupStore   map[string]string

	// oidc
	accessTokens           map[string]fosite.Requester
	accessTokenRequestIDs  map[string]string
	refreshTokens          map[string]db.StoreRefreshToken
	refreshTokenRequestIDs map[string]string
}

func (m *MemoryDataStore) GetAuthorIdsOfPadChats(id string) (*[]string, error) {
	var authorIds []string
	for k := range m.chatPads {
		if strings.HasPrefix(k, id+":") {
			chatMessage := m.chatPads[k]
			if chatMessage.AuthorId != nil {
				authorIds = append(authorIds, *chatMessage.AuthorId)
			}
		}
	}
	return &authorIds, nil
}

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

func (m *MemoryDataStore) SaveGroup(groupId string) error {
	m.groupStore[groupId] = groupId
	return nil
}

func (m *MemoryDataStore) RemoveGroup(groupId string) error {
	delete(m.groupStore, groupId)
	return nil
}

func (m *MemoryDataStore) GetRevisions(padId string, startRev int, endRev int) (*[]db.PadSingleRevision, error) {
	var _, ok = m.padStore[padId]

	if !ok {
		return nil, errors.New(PadDoesNotExistError)
	}

	revisions, ok := m.padRevisions[padId]
	revisionsToReturn := make([]db.PadSingleRevision, 0)

	if !ok {
		return &revisionsToReturn, nil
	}
	for rev := startRev; rev <= endRev; rev++ {

		revisionFromPad, ok := revisions[rev]
		if !ok {
			continue
		}

		var padSingleRevision = db.PadSingleRevision{
			PadId:     padId,
			RevNum:    rev,
			Pool:      revisionFromPad.Pool,
			Changeset: revisionFromPad.Changeset,
			AText:     revisionFromPad.AText,
			AuthorId:  revisionFromPad.AuthorId,
			Timestamp: revisionFromPad.Timestamp,
		}

		revisionsToReturn = append(revisionsToReturn, padSingleRevision)
	}
	return &revisionsToReturn, nil
}

func (m *MemoryDataStore) QueryPad(offset int, limit int, sortBy string, ascending bool, pattern string) (*db.PadDBSearchResult, error) {
	var padKeys []string
	for k := range m.padStore {
		padKeys = append(padKeys, k)
	}

	if pattern != "" {
		var filteredPadKeys []string
		for _, key := range padKeys {
			if strings.Contains(key, pattern) {
				filteredPadKeys = append(filteredPadKeys, key)
			}
		}
		padKeys = filteredPadKeys
	}

	if sortBy == "padName" {
		slices.Sort(padKeys)
	}
	if !ascending {
		slices.Reverse(padKeys)
	}

	padEnd := math.Min(float64(len(padKeys)), float64(offset+limit))
	padStart := math.Max(0, float64(offset))
	padsToSearch := padKeys[int(padStart):int(padEnd)]
	padSearch := make([]db.PadDBSearch, 0)

	for _, padKey := range padsToSearch {
		retrievedPad := m.padStore[padKey]
		padSearch = append(padSearch, db.PadDBSearch{
			Padname:        padKey,
			RevisionNumber: m.padStore[padKey].RevNum,
			LastEdited:     m.padRevisions[padKey][retrievedPad.RevNum].Timestamp,
		})
	}

	padSearchResult := db.PadDBSearchResult{
		TotalPads: len(padKeys),
		Pads:      padSearch,
	}
	return &padSearchResult, nil
}

func (m *MemoryDataStore) GetChatsOfPad(padId string, start int, end int) (*[]db.ChatMessageDBWithDisplayName, error) {
	var chatMessages []db.ChatMessageDBWithDisplayName
	for head := start; head <= end; head++ {
		var chatMessageKey = calcChatMessageKey(padId, head)
		chatMessage, ok := m.chatPads[chatMessageKey]
		if ok {
			var displayName *string
			if chatMessage.AuthorId != nil {
				authorId := *chatMessage.AuthorId
				if authorFromDB, ok := m.authorStore[authorId]; ok {

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

func (m *MemoryDataStore) SaveChatHeadOfPad(padId string, head int) error {
	var pad, ok = m.padStore[padId]

	if !ok {
		return errors.New(PadDoesNotExistError)
	}

	pad.ChatHead = head
	m.padStore[padId] = pad
	return nil
}

func calcChatMessageKey(padId string, head int) string {
	return padId + ":" + strconv.Itoa(head)
}

func (m *MemoryDataStore) SaveChatMessage(padId string, head int, authorId *string, timestamp int64, text string) error {
	var chatMessage = db.ChatMessageDB{
		PadId:    padId,
		Head:     head,
		AuthorId: authorId,
		Time:     &timestamp,
		Message:  text,
	}
	m.chatPads[calcChatMessageKey(padId, head)] = chatMessage
	return nil
}

func (m *MemoryDataStore) RemovePad(padID string) error {
	delete(m.padStore, padID)
	return nil
}

func (m *MemoryDataStore) RemoveRevisionsOfPad(padId string) error {
	var pad, ok = m.padStore[padId]

	if !ok {
		return errors.New(PadDoesNotExistError)
	}

	m.padRevisions[padId] = make(map[int]db.PadSingleRevision)
	pad.RevNum = -1
	m.padStore[padId] = pad
	return nil
}

func (m *MemoryDataStore) RemoveReadOnly2Pad(id string) error {
	delete(m.readonly2Pad, id)
	return nil
}

func (m *MemoryDataStore) RemovePad2ReadOnly(id string) error {
	delete(m.pad2Readonly, id)
	return nil
}

func (m *MemoryDataStore) RemoveChat(padId string) error {
	for k := range m.chatPads {
		if strings.HasPrefix(k, padId+":") {
			delete(m.chatPads, k)
		}
	}
	return nil
}

func (m *MemoryDataStore) GetGroup(groupId string) (*string, error) {
	group, ok := m.groupStore[groupId]
	if !ok {
		return nil, errors.New("group not found")
	}
	return &group, nil
}

func (m *MemoryDataStore) GetSessionById(sessionID string) (*session2.Session, error) {
	var retrievedSession, ok = m.sessionStore[sessionID]

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

func (m *MemoryDataStore) SetAuthorByToken(token string, author string) error {
	m.tokenStore[token] = author
	return nil
}

func (m *MemoryDataStore) GetRevision(padId string, rev int) (*db.PadSingleRevision, error) {
	var _, ok = m.padStore[padId]

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

	var padSingleRevision = db.PadSingleRevision{
		PadId:     padId,
		RevNum:    rev,
		Pool:      revisionFromPad.Pool,
		Changeset: revisionFromPad.Changeset,
		AText:     revisionFromPad.AText,
		AuthorId:  revisionFromPad.AuthorId,
		Timestamp: revisionFromPad.Timestamp,
	}

	return &padSingleRevision, nil
}

func (m *MemoryDataStore) GetPadMetaData(padId string, revNum int) (*db.PadMetaData, error) {
	var _, ok = m.padStore[padId]

	if !ok {
		return nil, errors.New(PadDoesNotExistError)
	}
	padRevs := m.padRevisions[padId]

	rev, ok := padRevs[revNum]
	if !ok {
		return nil, errors.New(PadRevisionNotFoundError)
	}

	return &db.PadMetaData{
		AuthorId:  rev.AuthorId,
		Timestamp: rev.Timestamp,
		PadPool: db.PadPool{
			NextNum:     rev.Pool.NextNum,
			NumToAttrib: rev.Pool.NumToAttrib,
		},
	}, nil
}

func (m *MemoryDataStore) GetPadIds() (*[]string, error) {
	var padIds []string
	for k := range m.padStore {
		padIds = append(padIds, k)
	}
	return &padIds, nil
}

func NewMemoryDataStore() *MemoryDataStore {
	return &MemoryDataStore{
		padRevisions:           make(map[string]map[int]db.PadSingleRevision),
		accessTokenRequestIDs:  make(map[string]string),
		accessTokens:           make(map[string]fosite.Requester),
		refreshTokenRequestIDs: make(map[string]string),
		refreshTokens:          make(map[string]db.StoreRefreshToken),
		padStore:               make(map[string]db.PadDB),
		authorStore:            make(map[string]db.AuthorDB),
		pad2Readonly:           make(map[string]string),
		readonly2Pad:           make(map[string]string),
		sessionStore:           make(map[string]session2.Session),
		tokenStore:             make(map[string]string),
		groupStore:             make(map[string]string),
		chatPads:               make(map[string]db.ChatMessageDB),
	}
}

func (m *MemoryDataStore) DoesPadExist(padID string) (*bool, error) {
	_, ok := m.padStore[padID]
	return &ok, nil
}

func (m *MemoryDataStore) CreatePad(padID string, padDB db.PadDB) error {
	m.padStore[padID] = padDB
	m.padRevisions[padID] = make(map[int]db.PadSingleRevision)
	return nil
}

func (m *MemoryDataStore) SaveRevision(padId string, rev int, changeset string,
	text db.AText, pool db.RevPool, authorId *string, timestamp int64) error {
	var retrievedPad, ok = m.padStore[padId]
	if !ok {
		return errors.New(PadDoesNotExistError)
	}
	retrievedPad.RevNum = rev

	if m.padRevisions[padId] == nil {
		m.padRevisions[padId] = make(map[int]db.PadSingleRevision)
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
	m.padStore[padId] = retrievedPad
	return nil
}

func (m *MemoryDataStore) GetPad(padID string) (*db.PadDB, error) {
	pad, ok := m.padStore[padID]

	if !ok {
		return nil, errors.New(PadDoesNotExistError)
	}

	return &pad, nil
}

func (m *MemoryDataStore) GetReadonlyPad(padId string) (*string, error) {
	pad, ok := m.pad2Readonly[padId]

	if !ok {
		return nil, errors.New(PadReadOnlyIdNotFoundError)
	}
	return &pad, nil
}

func (m *MemoryDataStore) CreatePad2ReadOnly(padId string, readonlyId string) error {
	m.pad2Readonly[padId] = readonlyId
	return nil
}

func (m *MemoryDataStore) CreateReadOnly2Pad(padId string, readonlyId string) error {
	m.readonly2Pad[readonlyId] = padId
	return nil
}

func (m *MemoryDataStore) GetReadOnly2Pad(id string) (*string, error) {
	res, ok := m.readonly2Pad[id]

	if !ok {
		return nil, nil
	}

	return &res, nil
}

func (m *MemoryDataStore) GetAuthor(author string) (*db.AuthorDB, error) {
	retrievedAuthor, ok := m.authorStore[author]

	if !ok {
		return nil, errors.New(AuthorNotFoundError)
	}

	return &retrievedAuthor, nil
}

func (m *MemoryDataStore) GetAuthorByToken(token string) (*string, error) {
	var author, ok = m.tokenStore[token]
	if !ok {
		return nil, errors.New(AuthorNotFoundError)
	}
	return &author, nil
}

func (m *MemoryDataStore) SaveAuthor(author db.AuthorDB) error {
	m.authorStore[author.ID] = author
	return nil
}

func (m *MemoryDataStore) SaveAuthorName(authorId string, authorName string) error {
	var retrievedAuthor, ok = m.authorStore[authorId]
	if !ok {
		return errors.New("author not found")
	}
	retrievedAuthor.Name = &authorName
	m.authorStore[authorId] = retrievedAuthor
	return nil
}

func (m *MemoryDataStore) SaveAuthorColor(authorId string, authorColor string) error {
	var retrievedAuthor, ok = m.authorStore[authorId]
	if !ok {
		return errors.New("author not found")
	}
	retrievedAuthor.ColorId = authorColor
	m.authorStore[authorId] = retrievedAuthor
	return nil
}

func (m *MemoryDataStore) Close() error {
	return nil
}

var _ DataStore = (*MemoryDataStore)(nil)
