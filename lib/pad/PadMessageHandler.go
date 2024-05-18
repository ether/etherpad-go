package pad

import (
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/ws"
	"regexp"
)

type AuthSession struct {
	PadID         string
	Token         string
	ReadOnlyPadId *string
	ReadOnly      bool
}

var padManager *Manager
var readOnlyManager *ReadOnlyManager
var authorManager *author.Manager
var colorRegEx *regexp.Regexp

func init() {
	padManager = &Manager{}
	readOnlyManager = &ReadOnlyManager{}
	colorRegEx, _ = regexp.Compile("^#(?:[0-9A-F]{3}){1,2}$")
}

func HandleClientReadyMessage(ready ws.ClientReady) {

	var authSession = AuthSession{
		PadID: ready.Data.PadID,
		Token: ready.Data.Token,
	}

	if !padManager.doesPadExist(ready.Data.PadID) {
		authSession.PadID = padManager.SanitizePadId(ready.Data.PadID)
	}

	var padIds = readOnlyManager.getIds(&authSession.PadID)
	authSession.PadID = padIds.PadId
	authSession.ReadOnlyPadId = &padIds.ReadOnlyPadId
	authSession.ReadOnly = padIds.ReadOnly

	if ready.Data.UserInfo.ColorId != nil && !colorRegEx.MatchString(*ready.Data.UserInfo.ColorId) {
		println("Invalid color id")
		ready.Data.UserInfo.ColorId = nil
	}

	if ready.Data.UserInfo.Name != nil {
		authorManager.SetAuthorName(authSession.PadID, *ready.Data.UserInfo.Name)
	}

	if ready.Data.UserInfo.ColorId != nil {
		authorManager.SetAuthorColor(authSession.PadID, *ready.Data.UserInfo.ColorId)
	}

	authorManager.GetAuthor()
}
