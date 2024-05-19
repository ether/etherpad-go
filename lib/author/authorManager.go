package author

import (
	"errors"
	"github.com/ether/etherpad-go/lib/db"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/utils"
	"math/rand"
	"time"
)

type Manager struct {
	db db.DataStore
}

type Author struct {
	Id        string
	Name      *string
	ColorId   int
	PadIDs    map[string]struct{}
	Timestamp int64
}

func (m *Manager) SetAuthorColor(author string, colorId string) {
	m.db.SaveAuthorColor(author, colorId)
}

func (m *Manager) mapAuthorWithDBKey(key string, value string) Author {
	author, err := m.db.GetAuthorByMapperKeyAndMapperValue(key, value)

	if err != nil {
		return m.CreateAuthor(nil)
	}

	return Author{
		Id:        author.ID,
		Timestamp: author.Timestamp,
		Name:      author.Name,
		ColorId:   author.ColorId,
		PadIDs:    author.PadIDs,
	}
}

/**
 * Returns the AuthorID for a mapper.
 * @param {String} authorMapper The mapper
 * @param {String} name The name of the author (optional)
 */
func (m *Manager) CreateAuthorIfNotExistsFor(authorMapper string, name *string) Author {
	var author = m.mapAuthorWithDBKey("mapper2author", authorMapper)

	if name != nil {
		m.SetAuthorName(author.Id, *name)
	}

	return author
}

func (m *Manager) CreateAuthor(name *string) Author {
	authorId := utils.RandomString(16)

	author := Author{
		Id:        authorId,
		Name:      name,
		PadIDs:    make(map[string]struct{}),
		ColorId:   rand.Intn(len(utils.ColorPalette)),
		Timestamp: time.Now().Unix(),
	}

	m.db.SaveAuthor(db2.AuthorDB{
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
	m.db.SaveAuthor(db2.AuthorDB{
		Name:    author.Name,
		ColorId: author.ColorId,
		PadIDs:  author.PadIDs,
	})
}

/**
 * Returns the name of the author
 * @param {String} author The id of the author
 */
func (m *Manager) GetAuthorName(authorId string) string {
	author, err := m.db.GetAuthor(authorId)

	if err != nil {
		return ""
	}

	return *author.Name
}

func (m *Manager) SetAuthorName(authorId string, authorName string) {

	m.db.SaveAuthorName(authorId, authorName)
}

/**
 * Returns an array of all pads this author contributed to
 * @param {String} authorID The id of the author
 */
func (m *Manager) ListPadsOfAuthor(authorId string) ([]string, error) {
	author, err := m.db.GetAuthor(authorId)

	if err != nil {
		return nil, errors.New("Author not found")
	}

	var pads []string

	for k := range author.PadIDs {
		pads = append(pads, k)
	}

	return pads, nil
}

func (m *Manager) addPad(authorId string, padId string) {
	retrievedAuthor, err := m.db.GetAuthor(authorId)

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
	retrievedAuthor, err := m.db.GetAuthor(authorId)

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

func (m *Manager) GetAuthor(authorId string) Author {
	author, err := m.db.GetAuthor(authorId)

	if err != nil {
		return Author{}
	}

	return Author{
		Id:        author.ID,
		Name:      author.Name,
		ColorId:   author.ColorId,
		PadIDs:    author.PadIDs,
		Timestamp: author.Timestamp,
	}
}
