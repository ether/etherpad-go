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

func (m *Manager) SetAuthorColor(author string, colorId string) error {
	return m.Db.SaveAuthorColor(author, colorId)
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

	return &Author{
		Id: authorId,
	}, nil
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

	return m.Db.SaveAuthorName(authorId, authorName)
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
	if _, err := m.Db.GetAuthor(authorId); err != nil {
		return errors.New(db.AuthorNotFoundError)
	}

	// Sever the token binding first, before touching anything else.
	if err := m.Db.RemoveTokenOfAuthor(authorId); err != nil {
		return err
	}

	// Zero the display identity. The token was already removed above, so
	// SaveAuthor's token-preservation has nothing left to preserve.
	if err := m.saveAuthor(Author{
		Id:        authorId,
		Name:      nil,
		ColorId:   "0",
		Timestamp: time.Now().Unix(),
	}); err != nil {
		return err
	}

	// Null authorship on chat messages the author posted.
	return m.Db.ClearChatAuthorship(authorId)
}

func (m *Manager) GetPadsOfAuthor(authorId string) (*[]string, error) {
	padIds, err := m.Db.GetPadIdsOfAuthor(authorId)
	if err != nil {
		return nil, err
	}
	return padIds, nil
}
