package author

import (
	"errors"
	"math/rand"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
)

type Manager struct {
	Db db.DataStore
}

func NewManager(db db.DataStore) *Manager {
	return &Manager{
		Db: db,
	}
}

// Author represents an Etherpad author
// @Description An author who can collaborate on pads
type Author struct {
	Id        string  `json:"id" example:"a.s8oes9dhwrvt0zif"`
	Name      *string `json:"name" example:"John Doe"`
	ColorId   string  `json:"colorId" example:"#ff0000"`
	Token     *string `json:"token,omitempty"`
	Timestamp int64   `json:"timestamp" example:"1704067200000"`
}

// ErrAuthorErased is returned for writes to an erased author: its identity
// must not come back through a session that was still connected.
var ErrAuthorErased = errors.New("author is erased")

// IsErased reports whether the author was erased (GDPR Art. 17). Unknown
// authors are not erased.
func (m *Manager) IsErased(authorId string) bool {
	row, err := m.Db.GetAuthorRow(authorId)
	return err == nil && row != nil && row.Erased
}

func (m *Manager) SetAuthorColor(author string, colorId string) error {
	if m.IsErased(author) {
		return ErrAuthorErased
	}
	if err := m.Db.SaveAuthorColor(author, colorId); err != nil {
		return err
	}
	m.touch(author)
	return nil
}

// nowMillis is the clock of lastSeen; tests replace it.
var nowMillis = func() int64 { return time.Now().UnixMilli() }

// touch records that the author was seen now (lastSeen: created, renamed,
// recoloured). A failure only loses the timestamp.
func (m *Manager) touch(authorId string) {
	_ = m.Db.TouchAuthor(authorId, nowMillis())
}

func (m *Manager) mapAuthorWithDBKey(token string) (*Author, error) {
	author, err := m.Db.GetAuthorByToken(token)

	if err != nil {
		// there is no author with this mapper, so create one
		var authorCreated, err = m.CreateAuthor(nil)
		if err != nil {
			return nil, err
		}
		// create the token2author relation
		err = m.Db.SetAuthorByToken(token, authorCreated.Id)

		if err != nil {
			return nil, err
		}

		// return the author
		return &Author{
			Id: authorCreated.Id,
		}, nil
	}

	// there is an author with this mapper
	// update the timestamp of this author
	return &Author{
		Id: *author,
	}, nil
}

func (m *Manager) CreateAuthor(name *string) (*Author, error) {
	authorId := "a." + utils.RandomString(16)

	author := Author{
		Id:        authorId,
		Name:      name,
		ColorId:   utils.ColorPalette[rand.Intn(len(utils.ColorPalette))],
		Timestamp: time.Now().Unix(),
	}

	if err := m.Db.SaveAuthor(MapToDB(author)); err != nil {
		return nil, err
	}
	m.touch(authorId)

	return &Author{
		Id: authorId,
	}, nil
}

// CreateAuthorIfNotExistsFor returns the author of an external id (the
// mapper, e.g. a user id of an LMS), creating it on first use, and sets its
// name when one is given (etherpad-lite's createAuthorIfNotExistsFor).
// Authors created by an older etherpad-go have the mapper as their token;
// they are found that way and get the mapper recorded.
func (m *Manager) CreateAuthorIfNotExistsFor(mapper string, name *string) (*Author, error) {
	id, err := m.Db.GetAuthorByMapper(mapper)
	if err != nil {
		return nil, err
	}
	if id == "" {
		if legacy, lerr := m.Db.GetAuthorByToken(mapper); lerr == nil && legacy != nil {
			id = *legacy
			if err := m.Db.SetAuthorMapper(id, mapper); err != nil {
				return nil, err
			}
		}
	}
	if id == "" {
		created, err := m.CreateAuthor(nil)
		if err != nil {
			return nil, err
		}
		id = created.Id
		if err := m.Db.SetAuthorMapper(id, mapper); errors.Is(err, db.ErrMapperTaken) {
			// A concurrent first use won: take its author. The one created
			// here has no token or mapper and stays unused.
			if id, err = m.Db.GetAuthorByMapper(mapper); err != nil {
				return nil, err
			}
			if id == "" {
				return nil, db.ErrMapperTaken
			}
		} else if err != nil {
			return nil, err
		}
	}
	if name != nil && *name != "" {
		if err := m.SetAuthorName(id, *name); err != nil {
			return nil, err
		}
	}
	return &Author{Id: id}, nil
}

func (m *Manager) saveAuthor(author Author) error {
	return m.Db.SaveAuthor(MapToDB(author))
}

/**
 * Returns the name of the author
 * @param {String} author The id of the author
 */
func (m *Manager) GetAuthorName(authorId string) (*string, error) {
	author, err := m.Db.GetAuthor(authorId)

	if err != nil {
		return nil, err
	}

	return author.Name, nil
}

func (m *Manager) SetAuthorName(authorId string, authorName string) error {
	if m.IsErased(authorId) {
		return ErrAuthorErased
	}
	if err := m.Db.SaveAuthorName(authorId, authorName); err != nil {
		return err
	}
	m.touch(authorId)
	return nil
}

func (m *Manager) GetAuthorId(token string) (*Author, error) {
	var res, err = m.GetAuthor4Token(token)

	if err != nil {
		return nil, err
	}

	return res, nil
}

func (m *Manager) GetAuthor4Token(token string) (*Author, error) {
	var author, err = m.mapAuthorWithDBKey(token)
	if err != nil {
		return nil, err
	}
	return author, nil
}

/**
 * Returns an array of all pads this author contributed to
 * @param {String} authorID The id of the author
 */
func (m *Manager) ListPadsOfAuthor(authorId string) ([]string, error) {
	_, err := m.Db.GetAuthor(authorId)

	if err != nil {
		return nil, errors.New(db.AuthorNotFoundError)
	}

	var pads []string

	padIds, err := m.Db.GetPadIdsOfAuthor(authorId)
	if err != nil {
		return nil, err
	}
	for _, k := range *padIds {
		pads = append(pads, k)
	}

	return pads, nil
}

func (m *Manager) GetAuthors(authorIds []string) (*[]Author, error) {
	var authors []Author
	dbAuthors, err := m.Db.GetAuthors(authorIds)
	if err != nil {
		return nil, err
	}
	for _, author := range *dbAuthors {
		authors = append(authors, MapFromDB(author))
	}
	return &authors, nil
}

func (m *Manager) GetAuthor(authorId string) (*Author, error) {
	author, err := m.Db.GetAuthor(authorId)

	if err != nil {
		return nil, err
	}

	mappedDbAuthor := MapFromDB(*author)
	return &mappedDbAuthor, nil
}

/**
 * AnonymizeAuthor performs GDPR Art. 17 erasure for an author, mirroring the
 * original Etherpad's AuthorManager.anonymizeAuthor (API 1.3.1):
 *   - the token binding that links a person to this author id is severed
 *     first, so a concurrent token lookup can no longer resolve the author
 *     mid-erasure,
 *   - the display identity on the author record is zeroed (name -> null,
 *     colorId -> 0) while the record itself is kept,
 *   - authorship on chat messages the author posted is nulled; the message
 *     text itself survives,
 *   - pad content, revisions and attribute pools are left intact: changeset
 *     references are opaque without the identity record.
 * The operation is idempotent: re-running it leaves the same erased state.
 * Returns db.AuthorNotFoundError if the author does not exist.
 * @param {String} authorId The id of the author
 */
func (m *Manager) AnonymizeAuthor(authorId string) error {
	_, err := m.Anonymize(authorId, false)
	return err
}

// ErasureResult counts what erasing an author touches (etherpad-lite's
// anonymizeAuthor counters).
type ErasureResult struct {
	AffectedPads            int `json:"affectedPads"`
	RemovedTokenMappings    int `json:"removedTokenMappings"`
	RemovedExternalMappings int `json:"removedExternalMappings"`
	ClearedChatMessages     int `json:"clearedChatMessages"`
}

// Anonymize erases an author (GDPR Art. 17): the token and the mapper that
// tie a person to the author are removed, name and colour zeroed, the
// author's chat messages lose their authorship, and the author is marked
// erased. Pad text and revisions stay; without the identity record their
// author ids are opaque. With dryRun nothing changes and the counters tell
// what would. An unknown author gives zero counters and
// db.AuthorNotFoundError, so callers can tell. A run that stopped before
// marking the author erased is completed by the next one. An author already
// erased is cleaned up again (a session still connected may have written a
// name, a colour or chat meanwhile); its counters then count only what was
// left, so a plain repeat gives zeros.
func (m *Manager) Anonymize(authorId string, dryRun bool) (ErasureResult, error) {
	var res ErasureResult
	a, err := m.Db.GetAuthor(authorId)
	if err != nil {
		return res, errors.New(db.AuthorNotFoundError)
	}
	row, err := m.Db.GetAuthorRow(authorId)
	if err != nil {
		return res, err
	}
	erased := row != nil && row.Erased
	if !erased {
		if pads, err := m.Db.GetPadIdsOfAuthor(authorId); err == nil && pads != nil {
			res.AffectedPads = len(*pads)
		}
	}
	if a.Token != nil && *a.Token != "" {
		res.RemovedTokenMappings = 1
	}
	if row != nil && row.Mapper != nil {
		res.RemovedExternalMappings = 1
	}
	if n, err := m.Db.CountChatMessagesOfAuthor(authorId); err == nil {
		res.ClearedChatMessages = n
	}
	if dryRun {
		return res, nil
	}
	if erased && res == (ErasureResult{}) && (a.Name == nil || *a.Name == "") && (a.ColorId == "" || a.ColorId == "0") {
		return res, nil // nothing came back
	}

	// Sever the bindings to the person first, before touching anything else.
	if err := m.Db.RemoveTokenOfAuthor(authorId); err != nil {
		return res, err
	}
	if _, err := m.Db.RemoveMapperOfAuthor(authorId); err != nil {
		return res, err
	}
	// Zero the display identity. The token was already removed above, so
	// SaveAuthor's token-preservation has nothing left to preserve.
	if err := m.saveAuthor(Author{
		Id:        authorId,
		Name:      nil,
		ColorId:   "0",
		Timestamp: time.Now().Unix(),
	}); err != nil {
		return res, err
	}
	// Null authorship on chat messages the author posted.
	if err := m.Db.ClearChatAuthorship(authorId); err != nil {
		return res, err
	}
	if erased {
		return res, nil // keeps the time of the first erasure
	}
	// Last: an interrupted run is not marked erased and runs again in full.
	return res, m.Db.MarkAuthorErased(authorId, nowMillis())
}

// AuthorListEntry is one author of the admin's list.
type AuthorListEntry struct {
	AuthorID string   `json:"authorID"`
	Name     *string  `json:"name"`
	ColorID  string   `json:"colorId"`
	Mapper   []string `json:"mapper"`
	LastSeen *int64   `json:"lastSeen"`
	Erased   bool     `json:"erased"`
	ErasedAt *int64   `json:"erasedAt"`
}

// AuthorSearchResult is a page of the admin's author list.
type AuthorSearchResult struct {
	Total    int               `json:"total"`
	Results  []AuthorListEntry `json:"results"`
	CappedAt *int              `json:"cappedAt"`
}

// authorSearchCap bounds the matches counted and paged through.
const authorSearchCap = 1000

// SearchAuthors pages through the authors matching pattern in id, name or
// mapper, erased ones only when asked (etherpad-lite's searchAuthors). At
// most authorSearchCap are considered; CappedAt says the cap was hit.
func (m *Manager) SearchAuthors(pattern string, offset, limit int, sortBy string, ascending, includeErased bool) (AuthorSearchResult, error) {
	if sortBy != "lastSeen" {
		sortBy = "name"
	}
	limit = min(max(limit, 1), 100)
	offset = max(offset, 0)
	rows, err := m.Db.SearchAuthors(db.AuthorSearch{
		Pattern: pattern, IncludeErased: includeErased, SortBy: sortBy, Ascending: ascending, Cap: authorSearchCap + 1,
	})
	if err != nil {
		return AuthorSearchResult{}, err
	}
	out := AuthorSearchResult{Results: []AuthorListEntry{}}
	if len(rows) > authorSearchCap {
		rows = rows[:authorSearchCap]
		c := authorSearchCap
		out.CappedAt = &c
	}
	out.Total = len(rows)
	for _, r := range rows[min(offset, len(rows)):min(offset+limit, len(rows))] {
		e := AuthorListEntry{AuthorID: r.ID, Name: r.Name, ColorID: r.ColorId, Mapper: []string{}, Erased: r.Erased}
		if r.Mapper != nil {
			e.Mapper = []string{*r.Mapper}
		}
		if r.LastSeen != 0 {
			ls := r.LastSeen
			e.LastSeen = &ls
		}
		if r.ErasedAt != 0 {
			ea := r.ErasedAt
			e.ErasedAt = &ea
		}
		out.Results = append(out.Results, e)
	}
	return out, nil
}

func (m *Manager) GetPadsOfAuthor(authorId string) (*[]string, error) {
	padIds, err := m.Db.GetPadIdsOfAuthor(authorId)
	if err != nil {
		return nil, err
	}
	return padIds, nil
}
