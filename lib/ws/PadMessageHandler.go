package ws

import (
	"encoding/json"
	"errors"
	"math"
	"regexp"
	"slices"
	"time"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	db2 "github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
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
	"go.uber.org/zap"
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

var colorRegEx *regexp.Regexp

type Task struct {
	socket  *Client
	message ws.UserChange
}

type ChannelOperator struct {
	channels map[string]chan Task
	handler  *PadMessageHandler
}

func NewChannelOperator(p *PadMessageHandler) ChannelOperator {
	return ChannelOperator{
		channels: make(map[string]chan Task),
		handler:  p,
	}
}

func (c *ChannelOperator) AddToQueue(ch string, t Task) {
	var _, ok = c.channels[ch]

	if !ok {
		c.channels[ch] = make(chan Task)
		go func() {
			for {
				var incomingTask = <-c.channels[ch]
				c.handler.handleUserChanges(incomingTask)
			}
		}()
	}

	c.channels[ch] <- t
}

type PadMessageHandler struct {
	padManager      *pad.Manager
	readOnlyManager *pad.ReadOnlyManager
	authorManager   *author.Manager
	securityManager pad.SecurityManager
	padChannels     ChannelOperator
	factory         clientVars.Factory
}

func NewPadMessageHandler(db db2.DataStore, hooks *hooks.Hook, padManager *pad.Manager) *PadMessageHandler {
	var padMessageHandler = PadMessageHandler{
		padManager:      padManager,
		readOnlyManager: pad.NewReadOnlyManager(db),
		authorManager:   author.NewManager(db),
		securityManager: pad.NewSecurityManager(db, hooks, padManager),
		factory: clientVars.Factory{
			ReadOnlyManager: pad.NewReadOnlyManager(db),
			AuthorManager:   author.NewManager(db),
		},
	}
	padMessageHandler.padChannels = NewChannelOperator(&padMessageHandler)
	return &padMessageHandler
}

func (p *PadMessageHandler) handleUserChanges(task Task) {
	var wireApool = apool.NewAPool()
	var newAPool = apool.NewAPool()
	newAPool.NextNum = task.message.Data.Data.Apool.NextNum
	newAPool.NumToAttribRaw = task.message.Data.Data.Apool.NumToAttrib
	wireApool = *wireApool.FromJsonable(newAPool)
	var session = SessionStoreInstance.getSession(task.socket.SessionId)
	if session == nil {
		println("Session not found for user changes")
		return
	}

	var retrievedPad, err = p.padManager.GetPad(session.PadId, nil, &session.Author)
	if err != nil {
		println("Error retrieving pad", err)
		return
	}
	_, err = changeset.CheckRep(task.message.Data.Data.Changeset)

	if err != nil {
		println("Error checking rep", err.Error())
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
		fromString := changeset.FromString(op.Attribs, &wireApool)
		var opAuthorId = fromString.Get("author")

		if opAuthorId != nil && utf8.RuneCountInString(*opAuthorId) != 0 && *opAuthorId != session.Author {
			println("Wrong author tried to submit changeset")
			return
		}
	}

	rebasedChangeset := changeset.MoveOpsToNewPool(task.message.Data.Data.Changeset, &wireApool, &retrievedPad.Pool)

	var r = task.message.Data.Data.BaseRev

	// The client's changeset might not be based on the latest revision,
	// since other clients are sending changes at the same time.
	// Update the changeset so that it can be applied to the latest revision.
	for r < retrievedPad.Head {
		r++
		var revisionPad, err = retrievedPad.GetRevision(r)
		if err != nil {
			println("Error retrieving revision", err)
			return
		}

		if revisionPad.Changeset == task.message.Data.Data.Changeset && revisionPad.AuthorId == &session.Author {
			// Assume this is a retransmission of an already applied changeset.
			unpackedChangeset, err = changeset.Unpack(task.message.Data.Data.Changeset)
			if err != nil {
				println("Error retrieving changeset", err)
				return
			}
			rebasedChangeset = changeset.Identity(unpackedChangeset.OldLen)
		}
		// At this point, both "c" (from the pad) and "changeset" (from the
		// client) are relative to revision r - 1. The follow function
		// rebases "changeset" so that it is relative to revision r
		// and can be applied after "c".
		rebasedChangeset = changeset.Follow(revisionPad.Changeset, rebasedChangeset, false, &retrievedPad.Pool)
	}

	prevText := retrievedPad.Text()

	if changeset.OldLen(rebasedChangeset) != utf8.RuneCountInString(prevText) {
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

	var correctionChangeset = p.correctMarkersInPad(retrievedPad.AText, retrievedPad.Pool)
	if correctionChangeset != nil {
		retrievedPad.AppendRevision(*correctionChangeset, &session.Author)
	}

	// Make sure the pad always ends with an empty line.
	if utils.RuneLastIndex(retrievedPad.Text(), "\n") != utf8.RuneCountInString(retrievedPad.Text())-1 {
		var nlChangeset, _ = changeset.MakeSplice(retrievedPad.Text(), utf8.RuneCountInString(retrievedPad.Text())-1, 0, "\n", nil, nil)
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
	retrievedPad.Head = newRev
	p.UpdatePadClients(retrievedPad)
}

func (p *PadMessageHandler) ComposePadChangesets(retrievedPad *pad2.Pad, startNum int, endNum int) (string, error) {
	headRev := retrievedPad.Head

	endNum = int(math.Min(float64(endNum), float64(headRev+1)))
	startNum = int(math.Max(float64(startNum), 0))

	changesetsNeeded := make([]int, 0)
	for i := startNum; i < endNum; i++ {
		changesetsNeeded = append(changesetsNeeded, i)
	}

	requiredChangesets, err := retrievedPad.GetRevisions(changesetsNeeded[0], changesetsNeeded[len(changesetsNeeded)-1])
	if err != nil {
		println("Error retrieving revisions for composing changesets", err)
		return "", err
	}
	startChangeset := (*requiredChangesets)[startNum].Changeset
	padPool := retrievedPad.Pool
	for r := startNum + 1; r < endNum; r++ {
		cs := (*requiredChangesets)[r]
		startChangeset = changeset.Compose(startChangeset, cs.Changeset, padPool)
	}
	return startChangeset, nil
}

func (p *PadMessageHandler) handleMessage(message any, client *Client, ctx *fiber.Ctx, retrievedSettings *settings.Settings, logger *zap.SugaredLogger) {
	var isSessionInfo = SessionStoreInstance.hasSession(client.SessionId)

	if !isSessionInfo {
		println("message from an unknown connection")
		return
	}

	castedMessage, ok := message.(ws.ClientReady)
	var thisSession = SessionStoreInstance.getSession(client.SessionId)

	if ok {
		thisSession = SessionStoreInstance.addHandleClientInformation(client.SessionId, castedMessage.Data.PadID, castedMessage.Data.Token)

		if !p.padManager.DoesPadExist(thisSession.Auth.PadId) {
			var padId, err = p.padManager.SanitizePadId(castedMessage.Data.PadID)

			if err != nil {
				println("Error sanitizing pad id", err.Error())
				return
			}
			thisSession.PadId = *padId
		}

		var padIds = p.readOnlyManager.GetIds(&thisSession.Auth.PadId)
		SessionStoreInstance.addPadReadOnlyIds(client.SessionId, padIds.PadId, padIds.ReadOnlyPadId, padIds.ReadOnly)
		thisSession = SessionStoreInstance.getSession(client.SessionId)
	}

	var auth = thisSession.Auth

	if auth == nil {
		var ip string
		if settings.Displayed.DisableIPLogging {
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

	var grantedAccess, err = p.securityManager.CheckAccess(&auth.PadId, &auth.SessionId, &auth.Token, user)

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
			p.HandleClientReadyMessage(expectedType, client, thisSessionNewRetrieved, retrievedSettings, logger)
			return
		}
	case ws.ChatMessage:
		{
			chatMessage := ws.FromObject(expectedType.Data.Data.Message)
			var currMillis = time.Now().UnixMilli()
			chatMessage.Time = &currMillis
			chatMessage.AuthorId = &thisSession.Author
			p.SendChatMessageToPadClients(thisSession, chatMessage)
		}
	case ws.UserChange:
		{
			if readonly {
				println("write attempt on read-only pad")
				return
			}

			p.padChannels.AddToQueue(client.Room, Task{
				message: expectedType,
				socket:  client,
			})
		}
	case ws.GetChatMessages:
		{
			if expectedType.Data.Data.Start < 0 {
				println("Invalid start for chat messages")
				return
			}

			if expectedType.Data.Data.End < 0 {
				println("Invalid end for chat messages")
				return
			}

			var count = expectedType.Data.Data.End - expectedType.Data.Data.Start
			if count < 0 || count > 100 {
				println("End must be greater than start for chat messages and no more than 100 messages can be requested at once")
				return
			}

			retrievedPad, err := p.padManager.GetPad(thisSession.PadId, nil, &thisSession.Author)
			if err != nil {
				println("Error retrieving pad for chat messages", err)
				return
			}
			chatMessages, err := retrievedPad.GetChatMessages(expectedType.Data.Data.Start, expectedType.Data.Data.End)
			if err != nil {
				println("Error retrieving chat messages", err)
				return
			}

			var convertedMessages = make([]ws.ChatMessageSendData, 0, len(*chatMessages))
			for _, msg := range *chatMessages {
				convertedMessages = append(convertedMessages, ws.ChatMessageSendData{
					Time:     msg.Time,
					Text:     msg.Message,
					UserId:   msg.AuthorId,
					UserName: nil,
				})
				if msg.DisplayName != nil && *msg.DisplayName != "" {
					convertedMessages[len(convertedMessages)-1].UserName = msg.DisplayName
				}
			}

			if err != nil {
				println("Error retrieving chat messages", err)
				return
			}

			var arr = make([]interface{}, 2)
			arr[0] = "message"
			arr[1] = ws.GetChatMessagesResponse{
				Type: "COLLABROOM",
				Data: struct {
					Type     string                   `json:"type"`
					Messages []ws.ChatMessageSendData `json:"messages"`
				}{Type: "CHAT_MESSAGES", Messages: convertedMessages},
			}
			var marshalled, _ = json.Marshal(arr)
			client.conn.WriteMessage(websocket.TextMessage, marshalled)
		}
	case UserInfoUpdate:
		{
			p.HandleUserInfoUpdate(expectedType, client)
		}
	case PadDelete:
		{
			p.HandlePadDelete(client, expectedType)
		}
	default:
		println("Unknown message type received")
	}
}

func (p *PadMessageHandler) SendChatMessageToPadClients(session *ws.Session, chatMessage ws.ChatMessageData) {
	var retrievedPad, err = p.padManager.GetPad(session.PadId, nil, chatMessage.AuthorId)
	if err != nil {
		println("Error retrieving pad for chat message", err)
		return
	}
	// pad.appendChatMessage() ignores the displayName property so we don't need to wait for
	// authorManager.getAuthorName() to resolve before saving the message to the database.
	retrievedPad.AppendChatMessage(chatMessage.AuthorId, *chatMessage.Time, chatMessage.Text)
	authorName, err := p.authorManager.GetAuthorName(*chatMessage.AuthorId)
	if err != nil {
		println("Error retrieving author name for chat message", err)
	}
	if authorName != nil && *authorName != "" {
		chatMessage.DisplayName = authorName
	}
	for _, socket := range p.GetRoomSockets(session.PadId) {
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		arr[1] = ws.ChatBroadCastMessage{
			Type: "COLLABROOM",
			Data: struct {
				Type    string                  `json:"type"`
				Message ws.ChatMessageSendEvent `json:"message"`
			}{Type: "CHAT_MESSAGE", Message: ws.ChatMessageSendEvent{
				Time:     chatMessage.Time,
				Text:     chatMessage.Text,
				UserId:   chatMessage.AuthorId,
				UserName: chatMessage.DisplayName,
			},
			},
		}

		var marshalledMessage, _ = json.Marshal(arr)

		err := socket.conn.WriteMessage(websocket.TextMessage, marshalledMessage)
		if err != nil {
			println("Error sending chat message to client", err)
		}
	}
}

func (p *PadMessageHandler) HandlePadDelete(client *Client, padDeleteMessage PadDelete) {
	var session = SessionStoreInstance.getSession(client.SessionId)

	if session == nil || session.Author == "" || session.PadId == "" {
		println("Session not ready")
		return
	}

	var retrievedPad = p.padManager.DoesPadExist(padDeleteMessage.Data.PadID)
	if !retrievedPad {
		println("Pad does not exist")
		return
	}
	var retrievedPadObj, err = p.padManager.GetPad(padDeleteMessage.Data.PadID, nil, nil)
	if err != nil {
		println("Error retrieving pad")
		return
	}
	// Only the one doing the first revision can delete the pad, otherwise people could troll a lot
	firstContributor, err := retrievedPadObj.GetRevisionAuthor(0)
	if err != nil {
		println("Error retrieving first contributor")
		return
	}

	if *firstContributor != session.Author {
		println("Only first contributor can delete the pad")
		return
	}

	err = p.DeletePad(retrievedPadObj.Id)
	if err != nil {
		println("Error deleting pad", err)
		return
	}
}

func (p *PadMessageHandler) DeletePad(padId string) error {
	var retrievedPad = p.padManager.DoesPadExist(padId)
	if !retrievedPad {
		println("Pad does not exist")
		return errors.New("pad does not exist")
	}
	var retrievedPadObj, err = p.padManager.GetPad(padId, nil, nil)
	if err != nil {
		println("Error retrieving pad")
		return err
	}
	retrievedPadObj.Remove()
	p.KickSessionsFromPad(retrievedPadObj.Id)
	// remove the readonly entries
	var readonlyId = p.readOnlyManager.GetReadOnlyId(retrievedPadObj.Id)
	err = p.readOnlyManager.RemoveReadOnlyPad(readonlyId, retrievedPadObj.Id)
	if err != nil {
		println("Error removing read-only pad mapping")
		return err
	}
	if err := retrievedPadObj.RemoveAllChats(); err != nil {
		println("Error removing all chats " + err.Error())
		return err
	}

	if err := retrievedPadObj.RemoveAllSavedRevisions(); err != nil {
		println("Error removing all saved revisions " + err.Error())
		return err
	}
	if err := p.padManager.RemovePad(retrievedPadObj.Id); err != nil {
		println("Error removing pad " + err.Error())
		return err
	}
	return nil
}

func (p *PadMessageHandler) HandleUserInfoUpdate(userInfo UserInfoUpdate, client *Client) {
	if userInfo.Data.UserInfo.ColorId == nil {
		return
	}

	if userInfo.Data.UserInfo.Name == nil {
		userInfo.Data.UserInfo.Name = nil
	}
	var session = SessionStoreInstance.getSession(client.SessionId)

	if session == nil || session.Author == "" || session.PadId == "" {
		println("Session not ready")
		return
	}

	var match, _ = regexp.MatchString("^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$", *userInfo.Data.UserInfo.ColorId)
	if !match {
		println("Malformed color", *userInfo.Data.UserInfo.ColorId)
		return
	}

	if userInfo.Data.UserInfo.ColorId != nil {
		p.authorManager.SetAuthorColor(session.Author, *userInfo.Data.UserInfo.ColorId)
	}
	if userInfo.Data.UserInfo.Name != nil {
		p.authorManager.SetAuthorName(session.Author, *userInfo.Data.UserInfo.Name)
	}
	var padId = session.PadId

	var padSockets = p.GetRoomSockets(padId)

	var userNewInfoDat = ws.UserNewInfoDat{
		UserId:  session.Author,
		Name:    userInfo.Data.UserInfo.Name,
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

func (p *PadMessageHandler) correctMarkersInPad(atext apool.AText, apool apool.APool) *string {
	var text = atext.Text

	// collect char positions of line markers (e.g. bullets) in new atext
	// that aren't at the start of a line
	var badMarkers = make([]int, 0)
	var offset = 0

	deserializedOps, _ := changeset.DeserializeOps(atext.Attribs)

	for _, op := range *deserializedOps {
		var attribs = changeset.FromString(op.Attribs, &apool)
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

	var builder = changeset.NewBuilder(utf8.RuneCountInString(text))

	for _, i := range badMarkers {
		builder.KeepText(text[offset:i], changeset.KeepArgs{}, nil)
		builder.Remove(1, 0)
		offset = i + 1
	}

	var stringifierBuilder = builder.ToString()
	return &stringifierBuilder
}

func (p *PadMessageHandler) HandleDisconnectOfPadClient(client *Client, settings *settings.Settings, logger *zap.SugaredLogger) {
	var thisSession = SessionStoreInstance.getSession(client.SessionId)
	if thisSession == nil || thisSession.PadId == "" {
		SessionStoreInstance.removeSession(client.SessionId)
		return
	}

	if settings.DisableIPLogging {
		logger.Infof("[LEAVE] pad:%s socket:%s IP:ANONYMOUS ", thisSession.PadId, client.SessionId)
	} else {
		logger.Infof("[LEAVE] pad:%s socket:%s IP:%s ", thisSession.PadId, client.SessionId, client.ctx.IP())
	}

	var roomSockets = p.GetRoomSockets(thisSession.PadId)
	var authorToRemove, err = p.authorManager.GetAuthor(thisSession.Author)
	if err != nil {
		println("Error retrieving author for disconnect")
		return
	}

	for _, otherSocket := range roomSockets {
		if otherSocket.SessionId == client.SessionId {
			continue
		}
		userLeave := ws.UserLeaveData{
			Type: "COLLABROOM",
			Data: struct {
				Type     string `json:"type"`
				UserInfo struct {
					ColorId string `json:"colorId"`
					UserId  string `json:"userId"`
				} `json:"userInfo"`
			}{Type: "USER_LEAVE", UserInfo: struct {
				ColorId string `json:"colorId"`
				UserId  string `json:"userId"`
			}{
				ColorId: authorToRemove.ColorId,
				UserId:  thisSession.Author,
			}},
		}
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		arr[1] = userLeave
		var marshalled, _ = json.Marshal(arr)
		if err := otherSocket.conn.WriteMessage(websocket.TextMessage, marshalled); err != nil {
			println("Error broadcasting USER_LEAVE message")
			return
		}
	}

	SessionStoreInstance.removeSession(client.SessionId)
}

func (p *PadMessageHandler) HandleClientReadyMessage(ready ws.ClientReady, client *Client, thisSession *ws.Session, retrievedSettings *settings.Settings, logger *zap.SugaredLogger) {
	if ready.Data.UserInfo.ColorId != nil && !colorRegEx.MatchString(*ready.Data.UserInfo.ColorId) {
		println("Invalid color id")
		ready.Data.UserInfo.ColorId = nil
	}

	if ready.Data.UserInfo.Name != nil {
		p.authorManager.SetAuthorName(thisSession.Author, *ready.Data.UserInfo.Name)
	}

	if ready.Data.UserInfo.ColorId != nil {
		p.authorManager.SetAuthorColor(thisSession.Author, *ready.Data.UserInfo.ColorId)
	}

	var retrievedPad, err = p.padManager.GetPad(thisSession.PadId, nil, &thisSession.Author)

	if err != nil {
		println("Error getting pad")
		return
	}

	var loggerStr = "pad:%s socket:%s"
	var argsForLogger = []interface{}{thisSession.PadId, client.SessionId}
	if retrievedPad.Head == 0 {
		loggerStr = "[CREATE] " + loggerStr
	} else {
		loggerStr = "[ENTER] " + loggerStr
	}

	if retrievedSettings.DisableIPLogging {
		loggerStr += " IP:ANONYMOUS "
	} else {
		loggerStr += " IP:%s "
		argsForLogger = append(argsForLogger, client.ctx.IP())
	}

	logger.Infof(loggerStr, argsForLogger...)

	var foundAuthor, errAuth = p.authorManager.GetAuthor(thisSession.Author)

	if errAuth != nil {
		println("Error retrieving author")
		return
	}

	if foundAuthor == nil || (*foundAuthor).Id == "" {
		println("Author not found")
		return
	}

	var authors = retrievedPad.GetAllAuthors()

	var _ = retrievedPad.GetPadMetaData(retrievedPad.Head)

	var historicalAuthorData = make(map[string]author.Author)

	for _, a := range authors {
		var retrievedAuthor, err = p.authorManager.GetAuthor(a)

		if err != nil {
			continue
		}

		historicalAuthorData[a] = *retrievedAuthor
	}

	var roomSockets = p.GetRoomSockets(thisSession.PadId)

	for _, otherSocket := range roomSockets {
		if otherSocket.SessionId == client.SessionId {
			continue
		}
		var sinfo = SessionStoreInstance.getSession(otherSocket.SessionId)
		if sinfo == nil {
			continue
		}

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
			Data: p.factory.NewClientVars(*retrievedPad, thisSession, wirePool, historicalAuthorData, retrievedSettings),
			Type: "CLIENT_VARS",
		}
		var encoded, _ = json.Marshal(arr)
		// Join the pad and start receiving updates
		thisSession.PadId = retrievedPad.Id
		// Send the clientVars to the Client
		client.conn.WriteMessage(websocket.TextMessage, encoded)
		// Save the current revision in sessioninfos, should be the same as in clientVars
		thisSession.Revision = retrievedPad.Head
	}

	retrievedAuthor, err := p.authorManager.GetAuthor(thisSession.Author)
	if err != nil {
		println("Error retrieving author for USER_NEWINFO broadcast")
		return
	}

	// Create and broadcast USER_NEWINFO message to all clients in the pad
	var userNewInfoDat = ws.UserNewInfoDat{
		UserId:  thisSession.Author,
		Name:    retrievedAuthor.Name,
		ColorId: retrievedAuthor.ColorId,
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

	for _, socket := range roomSockets {
		if err := socket.conn.WriteMessage(websocket.TextMessage, marshalled); err != nil {
			println("Error broadcasting USER_NEWINFO message")
		}
	}

	// send all other users' info to the new client
	for _, socket := range roomSockets {
		if socket.SessionId == client.SessionId {
			continue
		}
		var sinfo = SessionStoreInstance.getSession(socket.SessionId)
		if sinfo == nil {
			continue
		}
		otherAuthor, err := p.authorManager.GetAuthor(sinfo.Author)
		if err != nil {
			println("Error retrieving author for USER_NEWINFO send to new client")
			continue
		}
		var userNewInfoDat = ws.UserNewInfoDat{
			UserId:  sinfo.Author,
			Name:    otherAuthor.Name,
			ColorId: otherAuthor.ColorId,
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

		if err := client.conn.WriteMessage(websocket.TextMessage, marshalled); err != nil {
			println("Error sending USER_NEWINFO message to new client")
		}
	}
}

func (p *PadMessageHandler) UpdatePadClients(pad *pad2.Pad) {
	var roomSockets = p.GetRoomSockets(pad.Id)
	if len(roomSockets) == 0 {
		return
	}
	// since all clients usually get the same set of changesets, store them in local cache
	// to remove unnecessary roundtrip to the datalayer
	// NB: note below possibly now accommodated via the change to promises/async
	var revCache = make(map[int]*db.PadSingleRevision)
	for _, socket := range roomSockets {
		var sessionInfo = SessionStoreInstance.getSession(socket.SessionId)
		if sessionInfo == nil {
			continue
		}

		for sessionInfo.Revision < pad.Head {
			println("Sending NEW_CHANGES to client for pad", pad.Id, "from rev", sessionInfo.Revision, "to", pad.Head)
			var r = sessionInfo.Revision + 1
			if _, ok := revCache[r]; !ok {
				revCache[r], _ = pad.GetRevision(r)
			}
			var revision = revCache[r]
			var authorFromRev = revision.AuthorId
			var revChangeset = revision.Changeset
			var currentTime = revision.Timestamp

			var forWire = changeset.PrepareForWire(revChangeset, pad.Pool)
			var jsonAblePoolWithWire = forWire.Pool.ToJsonable()

			var arr = make([]interface{}, 2)
			arr[0] = "message"
			var msg = NewChangesMessage{
				Type: "COLLABROOM",
				Data: NewChangesMessageData{
					Type:        "NEW_CHANGES",
					NewRev:      r,
					Changeset:   forWire.Translated,
					APool:       jsonAblePoolWithWire,
					Author:      *authorFromRev,
					CurrentTime: currentTime,
					TimeDelta:   currentTime - sessionInfo.Time,
				},
			}
			arr[1] = msg

			marshalledMessage, err := json.Marshal(arr)

			if err != nil {
				println("Error sending NEW_CHANGES message to client")
				return
			}

			err = socket.conn.WriteMessage(websocket.TextMessage, marshalledMessage)
			sessionInfo.Time = currentTime
			sessionInfo.Revision = r
		}
	}
}

func (p *PadMessageHandler) GetRoomSockets(padID string) []Client {
	var sockets = make([]Client, 0)
	for k := range HubGlob.clients {
		sessId := SessionStoreInstance.getSession(k.SessionId)
		if sessId != nil && sessId.PadId == padID {
			sockets = append(sockets, *k)
		}
	}
	return sockets
}

func (p *PadMessageHandler) KickSessionsFromPad(padID string) {
	for k := range HubGlob.clients {
		if k == nil || k.SessionId == "" {
			continue
		}
		retrievedSession := SessionStoreInstance.getSession(k.SessionId)
		if retrievedSession == nil {
			continue
		}

		if retrievedSession.PadId == padID {
			k.SendPadDelete()
		}
	}
}
