package migration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// SQLDatabase implements the Database interface for SQL-based Etherpad stores
type SQLDatabase struct {
	db          *sql.DB
	tableName   string
	keyColumn   string
	valueColumn string
	placeholder func(n int) string // Returns placeholder like $1 or ? depending on driver
}

type DriverType int

const (
	DriverSQLite DriverType = iota
	DriverPostgres
	DriverMySQL
)

// NewSQLDatabase creates a new SQLDatabase with the appropriate placeholder style
func NewSQLDatabase(db *sql.DB, driver DriverType) *SQLDatabase {
	s := &SQLDatabase{
		db:          db,
		tableName:   "store",
		keyColumn:   "key",
		valueColumn: "value",
	}

	switch driver {
	case DriverPostgres:
		s.placeholder = func(n int) string { return fmt.Sprintf("$%d", n) }
	case DriverSQLite:
		s.placeholder = func(n int) string { return "?" }
	case DriverMySQL:
		s.placeholder = func(n int) string { return "?" }
		s.keyColumn = "`key`"
		s.valueColumn = "`value`"
	}

	return s
}

func (s *SQLDatabase) Close() error {
	return s.db.Close()
}

// ============================================================================
// Helper Methods
// ============================================================================

func (s *SQLDatabase) getValue(key string) (string, error) {
	query := fmt.Sprintf(
		"SELECT %s FROM %s WHERE %s = %s",
		s.valueColumn, s.tableName, s.keyColumn, s.placeholder(1),
	)

	var value string
	err := s.db.QueryRow(query, key).Scan(&value)
	if err != nil {
		return "", err
	}
	return value, nil
}

func (s *SQLDatabase) getKeysByPrefix(prefix string, lastKey string, limit int) ([]string, error) {
	var query string
	var args []interface{}

	if lastKey == "" {
		query = fmt.Sprintf(
			"SELECT %s FROM %s WHERE %s LIKE %s ORDER BY %s ASC LIMIT %s",
			s.keyColumn, s.tableName, s.keyColumn,
			s.placeholder(1), s.keyColumn, s.placeholder(2),
		)
		args = []interface{}{prefix + "%", limit}
	} else {
		query = fmt.Sprintf(
			"SELECT %s FROM %s WHERE %s LIKE %s AND %s > %s ORDER BY %s ASC LIMIT %s",
			s.keyColumn, s.tableName, s.keyColumn,
			s.placeholder(1), s.keyColumn, s.placeholder(2),
			s.keyColumn, s.placeholder(3),
		)
		args = []interface{}{prefix + "%", lastKey, limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	var keys []string
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		keys = append(keys, key)
	}

	return keys, rows.Err()
}

func (s *SQLDatabase) getKeysAndValuesByPrefix(
	prefix string,
	lastKey string,
	limit int,
) (map[string]string, error) {
	var query string
	var args []interface{}

	if lastKey == "" {
		query = fmt.Sprintf(
			"SELECT * FROM %s WHERE %s LIKE %s ORDER BY %s ASC LIMIT %s",
			s.tableName, s.keyColumn,
			s.placeholder(1), s.keyColumn, s.placeholder(2),
		)
		args = []interface{}{prefix + "%", limit}
	} else {
		query = fmt.Sprintf(
			"SELECT * FROM %s WHERE %s LIKE %s AND %s > %s ORDER BY %s ASC LIMIT %s",
			s.tableName, s.keyColumn,
			s.placeholder(1), s.keyColumn, s.placeholder(2),
			s.keyColumn, s.placeholder(3),
		)
		args = []interface{}{prefix + "%", lastKey, limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query failed: %w", err)
	}
	defer rows.Close()

	result := make(map[string]string)
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return nil, fmt.Errorf("scan failed: %w", err)
		}
		result[key] = value
	}

	return result, rows.Err()
}

// ============================================================================
// Pads
// ============================================================================

// Key pattern: pad:<padId>
var padKeyRegex = regexp.MustCompile(`^pad:([^:]+)$`)

func (s *SQLDatabase) GetNextPads(lastPadId string, limit int) ([]Pad, error) {
	lastKey := ""
	if lastPadId != "" {
		lastKey = "pad:" + lastPadId
	}

	data, err := s.getKeysAndValuesByPrefix("pad:", lastKey, limit*10) // Get extra to filter
	if err != nil {
		return nil, err
	}

	var pads []Pad
	for key, value := range data {
		matches := padKeyRegex.FindStringSubmatch(key)
		if matches == nil {
			continue // Skip revision keys like pad:xxx:revs:123
		}

		padId := matches[1]
		if lastPadId != "" && padId <= lastPadId {
			continue
		}

		var pad Pad
		if err := json.Unmarshal([]byte(value), &pad); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pad %s: %w", padId, err)
		}
		pad.PadId = padId
		pads = append(pads, pad)

		if len(pads) >= limit {
			break
		}
	}

	// Sort by PadId for consistent ordering
	sort.Slice(pads, func(i, j int) bool {
		return pads[i].PadId < pads[j].PadId
	})

	if len(pads) > limit {
		pads = pads[:limit]
	}

	return pads, nil
}

// ============================================================================
// Pad Revisions
// ============================================================================

// Key pattern: pad:<padId>:revs:<revNum>
func (s *SQLDatabase) GetPadRevisions(
	padId string,
	lastRev int,
	limit int,
) ([]PadRevision, error) {
	prefix := fmt.Sprintf("pad:%s:revs:", padId)

	// For revisions, we need to handle numeric ordering
	// Get all revision keys first, then filter and sort numerically
	keys, err := s.getKeysByPrefix(prefix, "", 100000) // Get all revisions
	if err != nil {
		return nil, err
	}

	// Parse and filter revision numbers
	type revKey struct {
		num int
		key string
	}
	var revKeys []revKey

	for _, key := range keys {
		numStr := strings.TrimPrefix(key, prefix)
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		if num > lastRev {
			revKeys = append(revKeys, revKey{num: num, key: key})
		}
	}

	// Sort by revision number
	sort.Slice(revKeys, func(i, j int) bool {
		return revKeys[i].num < revKeys[j].num
	})

	// Limit results
	if len(revKeys) > limit {
		revKeys = revKeys[:limit]
	}

	// Fetch values for selected keys
	var revisions []PadRevision
	for _, rk := range revKeys {
		value, err := s.getValue(rk.key)
		if err != nil {
			return nil, fmt.Errorf("failed to get revision %s: %w", rk.key, err)
		}

		var rev PadRevision
		if err := json.Unmarshal([]byte(value), &rev); err != nil {
			return nil, fmt.Errorf("failed to unmarshal revision %s: %w", rk.key, err)
		}
		rev.PadRevisionId = rk.key
		rev.RevNum = rk.num
		revisions = append(revisions, rev)
	}

	return revisions, nil
}

// ============================================================================
// Authors
// ============================================================================

// Key pattern: globalAuthor:<authorId>
func (s *SQLDatabase) GetNextAuthors(lastAuthorId string, limit int) ([]Author, error) {
	lastKey := ""
	if lastAuthorId != "" {
		lastKey = "globalAuthor:" + lastAuthorId
	}

	data, err := s.getKeysAndValuesByPrefix("globalAuthor:", lastKey, limit)
	if err != nil {
		return nil, err
	}

	var authors []Author
	for key, value := range data {
		authorId := strings.TrimPrefix(key, "globalAuthor:")

		var author Author
		if err := json.Unmarshal([]byte(value), &author); err != nil {
			return nil, fmt.Errorf("failed to unmarshal author %s: %w", authorId, err)
		}
		author.Id = authorId
		authors = append(authors, author)
	}

	// Sort for consistent ordering
	sort.Slice(authors, func(i, j int) bool {
		return authors[i].Id < authors[j].Id
	})

	return authors, nil
}

// ============================================================================
// Readonly Mappings
// ============================================================================

// GetNextReadonly2Pad Key pattern: readonly2pad:<readonlyId>
func (s *SQLDatabase) GetNextReadonly2Pad(
	lastReadonlyId string,
	limit int,
) ([]Readonly2Pad, error) {
	lastKey := ""
	if lastReadonlyId != "" {
		lastKey = "readonly2pad:" + lastReadonlyId
	}

	data, err := s.getKeysAndValuesByPrefix("readonly2pad:", lastKey, limit)
	if err != nil {
		return nil, err
	}

	var mappings []Readonly2Pad
	for key, value := range data {
		readonlyId := strings.TrimPrefix(key, "readonly2pad:")

		var padId string
		if err := json.Unmarshal([]byte(value), &padId); err != nil {
			return nil, fmt.Errorf("failed to unmarshal readonly2pad %s: %w", readonlyId, err)
		}

		mappings = append(mappings, Readonly2Pad{
			ReadonlyId: readonlyId,
			PadId:      padId,
		})
	}

	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].ReadonlyId < mappings[j].ReadonlyId
	})

	return mappings, nil
}

// Key pattern: pad2readonly:<padId>
func (s *SQLDatabase) GetNextPad2Readonly(lastPadId string, limit int) ([]Pad2Readonly, error) {
	lastKey := ""
	if lastPadId != "" {
		lastKey = "pad2readonly:" + lastPadId
	}

	data, err := s.getKeysAndValuesByPrefix("pad2readonly:", lastKey, limit)
	if err != nil {
		return nil, err
	}

	var mappings []Pad2Readonly
	for key, value := range data {
		padId := strings.TrimPrefix(key, "pad2readonly:")

		var readonlyId string
		if err := json.Unmarshal([]byte(value), &readonlyId); err != nil {
			return nil, fmt.Errorf("failed to unmarshal pad2readonly %s: %w", padId, err)
		}

		mappings = append(mappings, Pad2Readonly{
			PadId:      padId,
			ReadonlyId: readonlyId,
		})
	}

	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].PadId < mappings[j].PadId
	})

	return mappings, nil
}

// ============================================================================
// Token to Author
// ============================================================================

// Key pattern: token2author:<token>
func (s *SQLDatabase) GetNextToken2Author(lastToken string, limit int) ([]Token2Author, error) {
	lastKey := ""
	if lastToken != "" {
		lastKey = "token2author:" + lastToken
	}

	data, err := s.getKeysAndValuesByPrefix("token2author:", lastKey, limit)
	if err != nil {
		return nil, err
	}

	var mappings []Token2Author
	for key, value := range data {
		token := strings.TrimPrefix(key, "token2author:")

		var authorId string
		if err := json.Unmarshal([]byte(value), &authorId); err != nil {
			return nil, fmt.Errorf("failed to unmarshal token2author %s: %w", token, err)
		}

		mappings = append(mappings, Token2Author{
			Token:    token,
			AuthorId: authorId,
		})
	}

	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].Token < mappings[j].Token
	})

	return mappings, nil
}

// ============================================================================
// Chat Messages
// ============================================================================

// Key pattern: pad:<padId>:chat:<chatNum>
func (s *SQLDatabase) GetPadChatMessages(
	padId string,
	lastChatNum int,
	limit int,
) ([]ChatMessage, error) {
	prefix := fmt.Sprintf("pad:%s:chat:", padId)

	keys, err := s.getKeysByPrefix(prefix, "", 100000)
	if err != nil {
		return nil, err
	}

	type chatKey struct {
		num int
		key string
	}
	var chatKeys []chatKey

	for _, key := range keys {
		numStr := strings.TrimPrefix(key, prefix)
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		if num > lastChatNum {
			chatKeys = append(chatKeys, chatKey{num: num, key: key})
		}
	}

	sort.Slice(chatKeys, func(i, j int) bool {
		return chatKeys[i].num < chatKeys[j].num
	})

	if len(chatKeys) > limit {
		chatKeys = chatKeys[:limit]
	}

	var messages []ChatMessage
	for _, ck := range chatKeys {
		value, err := s.getValue(ck.key)
		if err != nil {
			return nil, fmt.Errorf("failed to get chat message %s: %w", ck.key, err)
		}

		var msg ChatMessage
		if err := json.Unmarshal([]byte(value), &msg); err != nil {
			return nil, fmt.Errorf("failed to unmarshal chat message %s: %w", ck.key, err)
		}
		msg.PadId = padId
		msg.ChatNum = ck.num
		messages = append(messages, msg)
	}

	return messages, nil
}

// ============================================================================
// Groups
// ============================================================================

// Key pattern: group:<groupId>
func (s *SQLDatabase) GetNextGroups(lastGroupId string, limit int) ([]Group, error) {
	lastKey := ""
	if lastGroupId != "" {
		lastKey = "group:" + lastGroupId
	}

	data, err := s.getKeysAndValuesByPrefix("group:", lastKey, limit)
	if err != nil {
		return nil, err
	}

	var groups []Group
	for key, value := range data {
		groupId := strings.TrimPrefix(key, "group:")

		var group Group
		if err := json.Unmarshal([]byte(value), &group); err != nil {
			return nil, fmt.Errorf("failed to unmarshal group %s: %w", groupId, err)
		}
		group.GroupId = groupId
		groups = append(groups, group)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].GroupId < groups[j].GroupId
	})

	return groups, nil
}

// Key pattern: group2sessions:<groupId>
func (s *SQLDatabase) GetNextGroup2Sessions(
	lastGroupId string,
	limit int,
) ([]Group2Sessions, error) {
	lastKey := ""
	if lastGroupId != "" {
		lastKey = "group2sessions:" + lastGroupId
	}

	data, err := s.getKeysAndValuesByPrefix("group2sessions:", lastKey, limit)
	if err != nil {
		return nil, err
	}

	var mappings []Group2Sessions
	for key, value := range data {
		groupId := strings.TrimPrefix(key, "group2sessions:")

		var sessions map[string]int
		if err := json.Unmarshal([]byte(value), &sessions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal group2sessions %s: %w", groupId, err)
		}

		mappings = append(mappings, Group2Sessions{
			GroupId:  groupId,
			Sessions: sessions,
		})
	}

	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].GroupId < mappings[j].GroupId
	})

	return mappings, nil
}

// Key pattern: author2sessions:<authorId>
func (s *SQLDatabase) GetNextAuthor2Sessions(
	lastAuthorId string,
	limit int,
) ([]Author2Sessions, error) {
	lastKey := ""
	if lastAuthorId != "" {
		lastKey = "author2sessions:" + lastAuthorId
	}

	data, err := s.getKeysAndValuesByPrefix("author2sessions:", lastKey, limit)
	if err != nil {
		return nil, err
	}

	var mappings []Author2Sessions
	for key, value := range data {
		authorId := strings.TrimPrefix(key, "author2sessions:")

		var sessions map[string]int
		if err := json.Unmarshal([]byte(value), &sessions); err != nil {
			return nil, fmt.Errorf("failed to unmarshal author2sessions %s: %w", authorId, err)
		}

		mappings = append(mappings, Author2Sessions{
			AuthorId: authorId,
			Sessions: sessions,
		})
	}

	sort.Slice(mappings, func(i, j int) bool {
		return mappings[i].AuthorId < mappings[j].AuthorId
	})

	return mappings, nil
}

// Key pattern: session:<sessionId>
func (s *SQLDatabase) GetNextSessions(lastSessionId string, limit int) ([]Session, error) {
	lastKey := ""
	if lastSessionId != "" {
		lastKey = "session:" + lastSessionId
	}

	data, err := s.getKeysAndValuesByPrefix("session:", lastKey, limit)
	if err != nil {
		return nil, err
	}

	var sessions []Session
	for key, value := range data {
		sessionId := strings.TrimPrefix(key, "session:")

		var session Session
		if err := json.Unmarshal([]byte(value), &session); err != nil {
			return nil, fmt.Errorf("failed to unmarshal session %s: %w", sessionId, err)
		}
		session.SessionId = sessionId
		sessions = append(sessions, session)
	}

	sort.Slice(sessions, func(i, j int) bool {
		return sessions[i].SessionId < sessions[j].SessionId
	})

	return sessions, nil
}
