package ws

import (
	"encoding/json"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	clientVars2 "github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/db"
	pad2 "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/models/webaccess"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/settings/clientVars"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gofiber/fiber/v2"
	"github.com/gorilla/websocket"
	"regexp"
	"slices"
	"strings"
)

type AuthSession struct {
	PadID         string
	Token         string
	ReadOnlyPadId *string
	ReadOnly      bool
}

type SessionInfo struct {
	sessionId string
	padId     string
}

var padManager pad.Manager
var readOnlyManager *pad.ReadOnlyManager
var authorManager author.Manager
var colorRegEx *regexp.Regexp
var securityManager pad.SecurityManager

func init() {
	padManager = pad.NewManager()
	readOnlyManager = pad.NewReadOnlyManager()
	authorManager = author.NewManager()
	colorRegEx, _ = regexp.Compile("^#(?:[0-9A-F]{3}){1,2}$")
	securityManager = pad.NewSecurityManager()
}

type Task struct {
	socket  *Client
	message ws.UserChange
}

var PadChannels = NewChannelOperator()

type ChannelOperator struct {
	channels map[string]chan Task
}

func NewChannelOperator() ChannelOperator {
	return ChannelOperator{
		channels: make(map[string]chan Task),
	}
}

func (c *ChannelOperator) AddToQueue(ch string, t Task) {
	var _, ok = PadChannels.channels[ch]

	if !ok {
		PadChannels.channels[ch] = make(chan Task)
		go func() {
			for {
				var incomingTask = <-PadChannels.channels[ch]
				handleUserChanges(incomingTask)
			}
		}()
	}

	PadChannels.channels[ch] <- t
}

func handleUserChanges(task Task) {
	var wireApool = apool.NewAPool()
	wireApool.FromJsonable(apool.APool{
		NextNum:        task.message.Data.Data.Apool.NextNum,
		NumToAttribRaw: task.message.Data.Data.Apool.NumToAttrib,
	})
	var session = SessionStoreInstance.getSession(task.socket.SessionId)

	var retrievedPad, _ = padManager.GetPad(session.PadId, nil, &session.Author)
	_, err := changeset.CheckRep(task.message.Data.Data.Changeset)

	if err != nil {
		return
	}

	unpackedChangeset, err := changeset.Unpack(task.message.Data.Data.Changeset)

	if err != nil {
		println("Error retrieving changeset", err)
	}
	deserializedOps, errWhenDeserializing := changeset.DeserializeOps(unpackedChangeset.Ops)

	if errWhenDeserializing != nil {
		println("error when deserializing ops")
		return
	}

	for _, op := range *deserializedOps {
		// + can add text with attribs
		// = can change or add attribs
		// - can have attribs, but they are discarded and don't show up in the attribs -
		// but do show up in the pool

		// Besides verifying the author attribute, this serves a second purpose:
		// AttributeMap.fromString() ensures that all attribute numbers are valid (it will throw if
		// an attribute number isn't in the pool).
		fromString := changeset.FromString(op.Attribs, *wireApool)
		var opAuthorId = fromString.Get("author")

		println(len(opAuthorId))

		if len(opAuthorId) != 0 && opAuthorId != session.Author {
			println("Wrong author tried to submit changeset")
			return
		}
	}

	rebasedChangeset := changeset.MoveOpsToNewPool(task.message.Data.Data.Changeset, *wireApool, retrievedPad.Pool)

	var r = task.message.Data.Data.BaseRev

	for r < retrievedPad.Head {
		r++
		var revisionPad, _ = retrievedPad.GetRevision(r)

		if revisionPad.Changeset == task.message.Data.Data.Changeset && revisionPad.AuthorId == &session.Author {
			// Assume this is a retransmission of an already applied changeset.
			unpackedChangeset, _ = changeset.Unpack(task.message.Data.Data.Changeset)
			rebasedChangeset = changeset.Identity(unpackedChangeset.OldLen)
		}
		// At this point, both "c" (from the pad) and "changeset" (from the
		// client) are relative to revision r - 1. The follow function
		// rebases "changeset" so that it is relative to revision r
		// and can be applied after "c".
		rebasedChangeset = changeset.Follow(revisionPad.Changeset, rebasedChangeset, false, &retrievedPad.Pool)
	}

	prevText := retrievedPad.Text()

	if changeset.OldLen(rebasedChangeset) != len(prevText) {
		panic("Can't apply changeset to pad text")
	}

	var newRev = retrievedPad.AppendRevision(rebasedChangeset, &session.Author)
	// The head revision will either stay the same or increase by 1 depending on whether the
	// changeset has a net effect.
	var rangeForRevs = make([]int, 2)
	rangeForRevs[0] = r
	rangeForRevs[1] = r + 1

	if !slices.Contains(rangeForRevs, newRev) {
		panic("Revision number is not within range")
	}

	var correctionChangeset = correctMarkersInPad(retrievedPad.AText, retrievedPad.Pool)
	if correctionChangeset != nil {
		retrievedPad.AppendRevision(*correctionChangeset, &session.Author)
	}

	// Make sure the pad always ends with an empty line.
	if strings.LastIndex(retrievedPad.Text(), "\n") != len(retrievedPad.Text())-1 {
		var nlChangeset, _ = changeset.MakeSplice(retrievedPad.Text(), len(retrievedPad.Text())-1, 0, "\n", nil, nil)
		retrievedPad.AppendRevision(nlChangeset, &session.Author)
	}

	if session.Revision != r {
		println("Revision mismatch")
	}

	// The client assumes that ACCEPT_COMMIT and NEW_CHANGES messages arrive in order. Make sure we
	// have already sent any previous ACCEPT_COMMIT and NEW_CHANGES messages.
	var arr = make([]interface{}, 2)
	arr[0] = "message"
	arr[1] = AcceptCommitMessage{
		Type: "COLLABROOM",
		Data: AcceptCommitData{
			Type:   "ACCEPT_COMMIT",
			NewRev: newRev,
		},
	}
	var bytes, _ = json.Marshal(arr)
	err = task.socket.conn.WriteMessage(websocket.TextMessage, bytes)
	if err != nil {
		println("error writing message")
		return
	}

	session.Revision = newRev

	if newRev != r {
		session.Time = retrievedPad.GetRevisionDate(newRev)
	}
	updatePadClients(retrievedPad)
}

func updatePadClients(pad *pad2.Pad) {
	var roomSockets = GetRoomSockets(pad.Id)
	if len(roomSockets) == 0 {
		return
	}
	// since all clients usually get the same set of changesets, store them in local cache
	// to remove unnecessary roundtrip to the datalayer
	// NB: note below possibly now accommodated via the change to promises/async
	// TODO: in REAL world, if we're working without datalayer cache,
	// all requests to revisions will be fired
	// BEFORE first result will be landed to our cache object.
	// The solution is to replace parallel processing
	// via async.forEach with sequential for() loop. There is no real
	// benefits of running this in parallel,
	// but benefit of reusing cached revision object is HUGE
	var revCache = make(map[int]*db.PadSingleRevision)

	for _, socket := range roomSockets {
		if !SessionStoreInstance.hasSession(socket.SessionId) {
			return
		}

		var sessionInfo = SessionStoreInstance.getSession(socket.SessionId)
		for sessionInfo.Revision < pad.Head {
			var r = sessionInfo.Revision + 1
			var revision, ok = revCache[r]
			if !ok {
				revCache[r], _ = pad.GetRevision(r)
				revision = revCache[r]
			}

			var authorString = revision.AuthorId
			var revChangeset = revision.Changeset
			var curentTime = revision.Timestamp
			var forWire = changeset.PrepareForWire(revChangeset, pad.Pool)

			var msg = NewChangesMessage{
				Type: "COLLABROOM",
				Data: NewChangesMessageData{
					Changeset:   forWire.Translated,
					Type:        "NEW_CHANGES",
					NewRev:      r,
					APool:       forWire.Pool,
					Author:      *authorString,
					CurrentTime: curentTime,
					TimeDelta:   curentTime - sessionInfo.Time,
				},
			}
			var arr = make([]interface{}, 2)
			arr[0] = "message"
			arr[1] = msg
			var newChangesMsg, _ = json.Marshal(arr)

			err := socket.conn.WriteMessage(websocket.TextMessage, newChangesMsg)

			if err != nil {
				println("Failed to notify user of new revision")
			}

		}
	}
}

func handleMessage(message any, client *Client, ctx *fiber.Ctx) {
	var isSessionInfo = SessionStoreInstance.hasSession(client.SessionId)

	if !isSessionInfo {
		println("message from an unknown connection")
		return
	}

	castedMessage, ok := message.(ws.ClientReady)
	var thisSession = SessionStoreInstance.getSession(client.SessionId)

	if ok {
		thisSession = SessionStoreInstance.addHandleClientInformation(client.SessionId, castedMessage.Data.PadID, castedMessage.Data.Token)

		if !padManager.DoesPadExist(thisSession.Auth.PadId) {
			var padId, err = padManager.SanitizePadId(castedMessage.Data.PadID)

			if err != nil {
				println("Error sanitizing pad id", err.Error())
				return
			}
			thisSession.PadId = *padId
		}

		var padIds = readOnlyManager.GetIds(&thisSession.Auth.PadId)
		SessionStoreInstance.addPadReadOnlyIds(client.SessionId, padIds.PadId, padIds.ReadOnlyPadId, padIds.ReadOnly)
		thisSession = SessionStoreInstance.getSession(client.SessionId)
	}

	var auth = thisSession.Auth

	if auth == nil {
		var ip string
		if settings.SettingsDisplayed.DisableIPLogging {
			ip = "ANONYMOUS"
		} else {
			ip = ctx.IP()
		}
		println("pre-CLIENT_READY message from IP " + ip)
		return
	}

	var user, okConv = ctx.Locals(clientVars2.WebAccessStore).(*webaccess.SocketClientRequest)

	if !okConv {
		user = nil
	}

	var grantedAccess, err = securityManager.CheckAccess(&auth.PadId, &auth.SessionId, &auth.Token, user)

	if err != nil {
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		arr[1] = AccessStatusMessage{
			AccessStatus: err.Error(),
		}
		var messageToSend, _ = json.Marshal(arr)

		client.conn.WriteMessage(websocket.TextMessage, messageToSend)
		println("Error checking access", err)
		return
	}

	if thisSession.Author != "" && thisSession.Author != grantedAccess.AuthorId {
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		arr[1] = UserDupMessage{
			Disconnect: "rejected",
		}
		var encoded, _ = json.Marshal(arr)
		client.conn.WriteMessage(websocket.TextMessage, encoded)
		return
	}

	thisSession.Author = grantedAccess.AuthorId

	var readonly = thisSession.ReadOnly
	var thisSessionNewRetrieved = SessionStoreInstance.getSession(client.SessionId)
	if thisSessionNewRetrieved == nil {
		println("Client disconnected")
		return
	}

	switch expectedType := message.(type) {
	case ws.ClientReady:
		{
			HandleClientReadyMessage(expectedType, client, thisSessionNewRetrieved)
			return
		}
	case ws.UserChange:
		{
			if readonly {
				println("write attempt on read-only pad")
				return
			}

			PadChannels.AddToQueue(client.Room, Task{
				message: expectedType,
				socket:  client,
			})
		}
	case UserInfoUpdate:
		{
			HandleUserInfoUpdate(expectedType, client)
		}
	}
}

func HandleUserInfoUpdate(userInfo UserInfoUpdate, client *Client) {
	if userInfo.Data.UserInfo.ColorId == nil {
		return
	}

	if userInfo.Data.UserInfo.Name == nil {
		userInfo.Data.UserInfo.Name = nil
	}
	var session = SessionStoreInstance.getSession(client.SessionId)

	if session == nil || session.Author == "" || session.PadId == "" {
		println("Session not ready")
	}

	var match, _ = regexp.MatchString("^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$", *userInfo.Data.UserInfo.ColorId)
	if !match {
		println("Malformed color")
		return
	}

	// Tell the authorManager about the new attributes
	var colorId int

	for i, color := range utils.ColorPalette {
		if *userInfo.Data.UserInfo.ColorId == color {
			colorId = i
		}
	}

	authorManager.SetAuthorColor(session.Author, colorId)
	authorManager.SetAuthorName(session.Author, *userInfo.Data.UserInfo.Name)
	var padId = session.PadId

	var padSockets = GetRoomSockets(padId)

	var userNewInfoDat = ws.UserNewInfoDat{
		UserId:  session.Author,
		Name:    *userInfo.Data.UserInfo.Name,
		ColorId: *userInfo.Data.UserInfo.ColorId,
	}

	var userNewInfo = ws.UserNewInfoData{
		Type:     "USER_NEWINFO",
		UserInfo: userNewInfoDat,
	}

	var userNewInfoActual = ws.UserNewInfo{
		Type: "COLLABROOM",
		Data: userNewInfo,
	}

	var arr = make([]interface{}, 2)
	arr[0] = "message"
	arr[1] = userNewInfoActual

	var marshalled, _ = json.Marshal(arr)

	for _, p := range padSockets {
		p.conn.WriteMessage(websocket.TextMessage, marshalled)
	}

}

func correctMarkersInPad(atext apool.AText, apool apool.APool) *string {
	var text = atext.Text

	// collect char positions of line markers (e.g. bullets) in new atext
	// that aren't at the start of a line
	var badMarkers = make([]int, 0)
	var offset = 0

	deserializedOps, _ := changeset.DeserializeOps(atext.Attribs)

	for _, op := range *deserializedOps {
		var attribs = changeset.FromString(op.Attribs, apool)
		var hasMarker = changeset.HasAttrib(attribs)

		if hasMarker {
			for i := 0; i < op.Chars; i++ {
				if offset > 0 && text[offset-1] != '\n' {
					badMarkers = append(badMarkers, offset)
				}
				offset++
			}
		} else {
			offset += op.Chars
		}
	}

	if len(badMarkers) == 0 {
		return nil
	}

	// create changeset that removes these bad markers
	offset = 0

	var builder = changeset.NewBuilder(len(text))

	for _, i := range badMarkers {
		builder.KeepText(text[offset:i], changeset.KeepArgs{}, nil)
		builder.Remove(1, 0)
		offset = i + 1
	}

	var stringifierBuilder = builder.ToString()
	return &stringifierBuilder
}

func HandleClientReadyMessage(ready ws.ClientReady, client *Client, thisSession *ws.Session) {
	if ready.Data.UserInfo.ColorId != nil && !colorRegEx.MatchString(*ready.Data.UserInfo.ColorId) {
		println("Invalid color id")
		ready.Data.UserInfo.ColorId = nil
	}

	if ready.Data.UserInfo.Name != nil {
		authorManager.SetAuthorName(thisSession.PadId, *ready.Data.UserInfo.Name)
	}

	var selectedColor = 0

	if ready.Data.UserInfo.ColorId != nil {
		for i, val := range utils.ColorPalette {
			if val == *ready.Data.UserInfo.ColorId {
				selectedColor = i
			}
		}

		authorManager.SetAuthorColor(thisSession.PadId, selectedColor)
	}

	var foundAuthor, errAuth = authorManager.GetAuthor(thisSession.Author)

	if errAuth != nil {
		println("Error retrieving author")
		return
	}

	var retrievedPad, err = padManager.GetPad(thisSession.PadId, nil, &foundAuthor.Id)
	if err != nil {
		println("Error getting pad")
		return
	}

	var authors = retrievedPad.GetAllAuthors()

	var _ = retrievedPad.GetPadMetaData(retrievedPad.Head)

	var historicalAuthorData = make(map[string]author.Author)

	for _, a := range authors {
		var retrievedAuthor, err = authorManager.GetAuthor(a)

		if err != nil {
			continue
		}

		historicalAuthorData[a] = *retrievedAuthor
	}

	var roomSockets = GetRoomSockets(thisSession.PadId)

	for _, otherSocket := range roomSockets {
		if otherSocket.SessionId == client.SessionId {
			continue
		}
		var sinfo = SessionStoreInstance.getSession(otherSocket.SessionId)

		if sinfo.Author == thisSession.Author {
			SessionStoreInstance.resetSession(otherSocket.SessionId)
			otherSocket.Leave()
			var arr = make([]interface{}, 2)
			arr[0] = "message"
			arr[1] = UserDupMessage{
				Disconnect: "userdup",
			}
			var encoded, _ = json.Marshal(arr)
			otherSocket.conn.WriteMessage(websocket.TextMessage, encoded)
		}
	}

	if ready.Data.Reconnect != nil && *ready.Data.Reconnect {

	} else {
		var atext = changeset.CloneAText(retrievedPad.AText)
		var attribsForWire = changeset.PrepareForWire(atext.Attribs, retrievedPad.Pool)
		atext.Attribs = attribsForWire.Translated
		wirePool := attribsForWire.Pool.ToJsonable()
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		arr[1] = Message{
			Data: clientVars.NewClientVars(*retrievedPad, thisSession, wirePool),
			Type: "CLIENT_VARS",
		}
		var encoded, _ = json.Marshal(arr)
		client.conn.WriteMessage(websocket.TextMessage, encoded)

	}
}

func GetRoomSockets(padID string) []Client {
	var sockets = make([]Client, 0)
	for k := range HubGlob.clients {
		if SessionStoreInstance.getSession(k.SessionId).PadId == padID {
			sockets = append(sockets, *k)
		}
	}
	return sockets
}
