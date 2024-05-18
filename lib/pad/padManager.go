package pad

import (
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/db"
	"regexp"
)

var padRegex *regexp.Regexp

func init() {
	padRegex, _ = regexp.Compile("^(g.[a-zA-Z0-9]{16}$)?[^$]{1,50}$")
}

type Manager struct {
	store db.DataStore
}

func (m *Manager) doesPadExist(padID string) bool {
	m.store.DoesPadExist(padID)
}

func (m *Manager) createPad(padID string) bool {
	if !m.store.CreatePad(padID) {

	}
	return true
}

func (m *Manager) SanitizePadId(padID string) string {
	if padRegex.MatchString(padID) {
		return padID
	}
	return padID
}

func (m *Manager) GetPad(padID string, text *string, author *author.Author) string {

}
