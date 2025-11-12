package author

import (
	"errors"
	"math/rand"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	db2 "github.com/ether/etherpad-go/lib/models/db"
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
	PadIDs    map[string]struct{}
	Timestamp int64
}

func (m *Manager) SetAuthorColor(author string, colorId string) {
	m.Db.SaveAuthorColor(author, colorId)
}

func (m *Manager) mapAuthorWithDBKey(token string) *Author {
	author, err := m.Db.GetAuthorByToken(token)

	if err != nil {
		// there is no author with this mapper, so create one
		var authorCreated = m.CreateAuthor(nil)
		// create the token2author relation
		err = m.Db.SetAuthorByToken(token, authorCreated.Id)

		if err != nil {
			panic(err.Error())
		}

		// return the author
		return &Author{
			Id: authorCreated.Id,
		}
	}

	// there is an author with this mapper
	// update the timestamp of this author
	return &Author{
		Id: *author,
	}
}

/**
 * Returns the AuthorID for a mapper.
 * @param {String} authorMapper The mapper
 * @param {String} name The name of the author (optional)
 */
/*func (m *Manager) CreateAuthorIfNotExistsFor(authorMapper string, name *string) Author {
	var author = m.mapAuthorWithDBKey("mapper2author", authorMapper)

	if name != nil {
		m.SetAuthorName(author.Id, *name)
	}

	return author
}*/

func (m *Manager) CreateAuthor(name *string) Author {
	authorId := "a." + utils.RandomString(16)

	author := Author{
		Id:        authorId,
		Name:      name,
		PadIDs:    make(map[string]struct{}),
		ColorId:   utils.ColorPalette[rand.Intn(len(utils.ColorPalette))],
		Timestamp: time.Now().Unix(),
	}

	m.Db.SaveAuthor(db2.AuthorDB{
		ID:        author.Id,
		Name:      author.Name,
		ColorId:   author.ColorId,
		PadIDs:    author.PadIDs,
		Timestamp: author.Timestamp,
	})

	return Author{
		Id: authorId,
	}
}

func (m *Manager) saveAuthor(author Author) {
	m.Db.SaveAuthor(db2.AuthorDB{
		Name:    author.Name,
		ColorId: author.ColorId,
		PadIDs:  author.PadIDs,
	})
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

func (m *Manager) SetAuthorName(authorId string, authorName string) {

	m.Db.SaveAuthorName(authorId, authorName)
}

func (m *Manager) GetAuthorId(token string) *Author {
	var res = m.GetAuthor4Token(token)

	return res
}

func (m *Manager) GetAuthor4Token(token string) *Author {
	var author = m.mapAuthorWithDBKey(token)
	return author
}

/**
 * Returns an array of all pads this author contributed to
 * @param {String} authorID The id of the author
 */
func (m *Manager) ListPadsOfAuthor(authorId string) ([]string, error) {
	author, err := m.Db.GetAuthor(authorId)

	if err != nil {
		return nil, errors.New("Author not found")
	}

	var pads []string

	for k := range author.PadIDs {
		pads = append(pads, k)
	}

	return pads, nil
}

func (m *Manager) AddPad(authorId string, padId string) {
	retrievedAuthor, err := m.Db.GetAuthor(authorId)

	if err != nil {
		return
	}

	if retrievedAuthor.PadIDs == nil {
		retrievedAuthor.PadIDs = make(map[string]struct{})
	}

	// add the entry for this pad
	retrievedAuthor.PadIDs[padId] = struct{}{} // anything, because value is not used

	m.saveAuthor(Author{
		Name:    retrievedAuthor.Name,
		ColorId: retrievedAuthor.ColorId,
		PadIDs:  retrievedAuthor.PadIDs,
	})
}

/**
 * Removes a pad from the list of contributions
 * @param {String} authorID The id of the author
 * @param {String} padID The id of the pad the author contributes to
 */
func (m *Manager) removePad(authorId string, padId string) {
	retrievedAuthor, err := m.Db.GetAuthor(authorId)

	if err != nil {
		return
	}

	var futurePadIDs map[string]struct{}

	if retrievedAuthor.PadIDs != nil {
		for k := range retrievedAuthor.PadIDs {
			if k != padId {
				futurePadIDs[k] = struct{}{}
			}
		}
	}

	retrievedAuthor.PadIDs = futurePadIDs

	m.saveAuthor(Author{
		Name:    retrievedAuthor.Name,
		ColorId: retrievedAuthor.ColorId,
		PadIDs:  retrievedAuthor.PadIDs,
	})
}

func (m *Manager) GetAuthor(authorId string) (*Author, error) {
	author, err := m.Db.GetAuthor(authorId)

	if err != nil {
		return nil, err
	}

	return &Author{
		Id:        author.ID,
		Name:      author.Name,
		ColorId:   author.ColorId,
		PadIDs:    author.PadIDs,
		Timestamp: author.Timestamp,
	}, nil
}
