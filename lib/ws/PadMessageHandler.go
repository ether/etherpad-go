package ws

import (
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/pad"
	"regexp"
)

type AuthSession struct {
	PadID         string
	Token         string
	ReadOnlyPadId *string
	ReadOnly      bool
}

var padManager pad.Manager
var readOnlyManager *pad.ReadOnlyManager
var authorManager *author.Manager
var colorRegEx *regexp.Regexp

func init() {
	padManager = pad.NewManager()
	readOnlyManager = pad.NewReadOnlyManager()
	colorRegEx, _ = regexp.Compile("^#(?:[0-9A-F]{3}){1,2}$")
}

func HandleClientReadyMessage(ready ws.ClientReady, client *Client) {

	var sessionInfo = SessionStore[client.SessionId]
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

	var retrievedPad, _ = padManager.GetPad(authSession.PadID, nil, &foundAuthor)

	var authors = retrievedPad.GetAllAuthors()

	var _ = retrievedPad.GetPadMetaData(retrievedPad.Head)

	var historicalAuthorData = make(map[string]author.Author, 0)

	for _, a := range authors {
		var retrievedAuthor = authorManager.GetAuthor(a)
		historicalAuthorData[a] = retrievedAuthor
	}

	var roomSockets = GetRoomSockets(authSession.PadID)

	for _, socket := range roomSockets {
		if socket.SessionId == client.SessionId {
			var sinfo = SessionStore[socket.SessionId]
			if sinfo.Author == sessionInfo.Author {
				SessionStore[socket.SessionId] = Session{}

				client.Leave()
			}
		}
	}
}

func GetRoomSockets(padID string) []Client {
	var sockets = make([]Client, 0)
	for k := range HubGlob.clients {
		if SessionStore[k.SessionId].PadId == padID {
			sockets = append(sockets, *k)
		}
	}
	return sockets
}
