package ws

import (
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/utils"
	"regexp"
)

type AuthSession struct {
	PadID         string
	Token         string
	ReadOnlyPadId *string
	ReadOnly      bool
}

var padManager *pad.Manager
var readOnlyManager *pad.ReadOnlyManager
var authorManager *author.Manager
var colorRegEx *regexp.Regexp

func init() {
	padManager = &pad.Manager{}
	readOnlyManager = &pad.ReadOnlyManager{}
	colorRegEx, _ = regexp.Compile("^#(?:[0-9A-F]{3}){1,2}$")
}

func HandleClientReadyMessage(ready ws.ClientReady, client pad.ClientType) {
	var sessionInfo = utils.SessionStore[client.(Client).SessionId]
	var authSession = AuthSession{
		PadID: ready.Data.PadID,
		Token: ready.Data.Token,
	}

	if !padManager.DoesPadExist(ready.Data.PadID) {
		authSession.PadID = padManager.SanitizePadId(ready.Data.PadID)
	}

	var padIds = readOnlyManager.GetIds(&authSession.PadID)
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

	var foundAuthor = authorManager.GetAuthor(sessionInfo.Author)

	var _, _ = padManager.GetPad(authSession.PadID, nil, &foundAuthor)
}
