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

type Author struct {
	Id        string
	Name      *string
	ColorId   string
	Token     *string
	Timestamp int64
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
