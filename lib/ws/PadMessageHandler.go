package ws

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"regexp"
	"slices"
	"strconv"
	"sync"
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
	"github.com/ether/etherpad-go/lib/ws/constants"
	"github.com/gofiber/fiber/v2"
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

func init() {
	colorRegEx = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
}

type Task struct {
	socket  *Client
	message ws.UserChange
}

type ChannelOperator struct {
	channels map[string]chan Task
	handler  *PadMessageHandler
	mu       sync.Mutex
}

func NewChannelOperator(p *PadMessageHandler) ChannelOperator {
	return ChannelOperator{
		channels: make(map[string]chan Task),
		handler:  p,
	}
}

func (c *ChannelOperator) AddToQueue(ch string, t Task) {
	c.mu.Lock()
	chChan, ok := c.channels[ch]
	if !ok {
		// small buffer to decouple producer from goroutine scheduling
		chChan = make(chan Task, 1)
		c.channels[ch] = chChan
		go func(localCh chan Task) {
			for incomingTask := range localCh {
				c.handler.handleUserChanges(incomingTask)
			}
		}(chChan)
	}
	c.mu.Unlock()

	chChan <- t
}

type PadMessageHandler struct {
	padManager      *pad.Manager
	readOnlyManager *pad.ReadOnlyManager
	authorManager   *author.Manager
	securityManager *pad.SecurityManager
	padChannels     ChannelOperator
	factory         clientVars.Factory
	SessionStore    *SessionStore
	hub             *Hub
	Logger          *zap.SugaredLogger
}

func NewPadMessageHandler(db db2.DataStore, hooks *hooks.Hook, padManager *pad.Manager, sessionStore *SessionStore, hub *Hub, logger *zap.SugaredLogger) *PadMessageHandler {
	var padMessageHandler = PadMessageHandler{
		padManager:      padManager,
		readOnlyManager: pad.NewReadOnlyManager(db),
		authorManager:   author.NewManager(db),
		securityManager: pad.NewSecurityManager(db, hooks, padManager),
		factory: clientVars.Factory{
			ReadOnlyManager: pad.NewReadOnlyManager(db),
			AuthorManager:   author.NewManager(db),
		},
		SessionStore: sessionStore,
		hub:          hub,
		Logger:       logger,
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
	var session = p.SessionStore.getSession(task.socket.SessionId)
	if session == nil {
		p.Logger.Infof("Session %s not found", task.socket.SessionId)
		return
	}

	var retrievedPad, err = p.padManager.GetPad(session.PadId, nil, &session.Author)
	if err != nil {
		println("Error retrieving pad", err)
		return
	}
	checkedRep, err := changeset.CheckRep(task.message.Data.Data.Changeset)

	if err != nil {
		println("Error checking rep", err.Error())
		return
	}

	unpackedChangeset, err := changeset.Unpack(*checkedRep)

	if err != nil {
		println("Error retrieving changeset", err)
		return
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
	headRev := retrievedPad.Head

	p.Logger.Debugf("Processing USER_CHANGES: baseRev=%d, headRev=%d, changeset=%s", r, headRev, task.message.Data.Data.Changeset)

	// The client's changeset might not be based on the latest revision,
	// since other Clients are sending changes at the same time.
	// Update the changeset so that it can be applied to the latest revision.
	for r < retrievedPad.Head {
		r++
		var revisionPad, err = retrievedPad.GetRevision(r)
		if err != nil {
			p.Logger.Warnf("Error retrieving revision %d: %v", r, err)
			return
		}

		if revisionPad.Changeset == task.message.Data.Data.Changeset && revisionPad.AuthorId == &session.Author {
			// Assume this is a retransmission of an already applied changeset.
			unpackedChangeset, err = changeset.Unpack(task.message.Data.Data.Changeset)
			if err != nil {
				p.Logger.Warnf("Error unpacking changeset: %v", err)
				return
			}
			rebasedChangeset = changeset.Identity(unpackedChangeset.OldLen)
		}
		// At this point, both "c" (from the pad) and "changeset" (from the
		// client) are relative to revision r - 1. The follow function
		// rebases "changeset" so that it is relative to revision r
		// and can be applied after "c".
		optRebasedChangeset, err := changeset.Follow(revisionPad.Changeset, rebasedChangeset, false, &retrievedPad.Pool)
		if err != nil {
			p.Logger.Warnf("Error rebasing changeset at rev %d: %v for %s", r, err, retrievedPad.Id)
			return
		}
		rebasedChangeset = *optRebasedChangeset
	}

	p.Logger.Debugf("After rebasing: rebasedChangeset=%s", rebasedChangeset)

	prevText := retrievedPad.Text()
	oldLen, err := changeset.OldLen(rebasedChangeset)

	if err != nil {
		p.Logger.Warnf("Error retrieving old len from changeset: %v", err)
		return
	}

	if *oldLen != utf8.RuneCountInString(prevText) {
		p.Logger.Warnf("Can't apply changeset to pad text: oldLen=%d, prevTextLen=%d, baseRev=%d, headRev=%d",
			*oldLen, utf8.RuneCountInString(prevText), r, retrievedPad.Head)
		return
	}

	newRev, err := retrievedPad.AppendRevision(rebasedChangeset, &session.Author)
	if err != nil {
		println("Error appending revision", err.Error())
		return
	}
	// The head revision will either stay the same or increase by 1 depending on whether the
	// changeset has a net effect.
	var rangeForRevs = make([]int, 2)
	rangeForRevs[0] = r
	rangeForRevs[1] = r + 1

	if !slices.Contains(rangeForRevs, *newRev) {
		p.Logger.Warnf("Head revision after appending changeset is unexpected. Expected: %v, Got: %d", rangeForRevs, *newRev)
		return
	}
	finalRev := retrievedPad.Head

	// The client assumes that ACCEPT_COMMIT and NEW_CHANGES messages arrive in order. Make sure we
	// have already sent any previous ACCEPT_COMMIT and NEW_CHANGES messages.
	var arr = make([]interface{}, 2)
	arr[0] = "message"
	arr[1] = AcceptCommitMessage{
		Type: "COLLABROOM",
		Data: AcceptCommitData{
			Type:   "ACCEPT_COMMIT",
			NewRev: finalRev,
		},
	}
	var bytes, _ = json.Marshal(arr)
	task.socket.SafeSend(bytes)

	session.Revision = finalRev

	if finalRev != r {
		optTime, err := retrievedPad.GetRevisionDate(finalRev)
		if err != nil {
			p.Logger.Warnf("Error retrieving revision date: %v", err)
			return
		}
		session.Time = *optTime
	}
	retrievedPad.Head = finalRev
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
		p.Logger.Warnf("Error retrieving required changesets: %v", err)
		return "", err
	}
	startChangeset := (*requiredChangesets)[startNum].Changeset
	padPool := retrievedPad.Pool
	for r := startNum + 1; r < endNum; r++ {
		cs := (*requiredChangesets)[r]
		optStartChangeset, err := changeset.Compose(startChangeset, cs.Changeset, &padPool)
		if err != nil {
			println("Error composing changesets", err)
			return "", err
		}
		startChangeset = *optStartChangeset
	}
	return startChangeset, nil
}

func (p *PadMessageHandler) HandleMessage(message any, client *Client, ctx *fiber.Ctx, retrievedSettings *settings.Settings, logger *zap.SugaredLogger) {
	var isSessionInfo = p.SessionStore.hasSession(client.SessionId)

	if !isSessionInfo {
		p.Logger.Warnf("No session info for client session ID: %s", client.SessionId)
		return
	}

	castedMessage, ok := message.(ws.ClientReady)
	var thisSession = p.SessionStore.getSession(client.SessionId)

	if ok {
		thisSession = p.SessionStore.addHandleClientInformation(client.SessionId, castedMessage.Data.PadID, castedMessage.Data.Token)
		exists, err := p.padManager.DoesPadExist(thisSession.Auth.PadId)

		if err != nil {
			p.Logger.Warnf("Error checking if pad exists: %v", err)
			return
		}

		if !*exists {
			var padId, err = p.padManager.SanitizePadId(castedMessage.Data.PadID)

			if err != nil {
				p.Logger.Warnf("Error sanitizing pad ID: %v", err)
				return
			}
			thisSession.PadId = *padId
		}

		padIds, err := p.readOnlyManager.GetIds(&thisSession.Auth.PadId)
		if err != nil {
			p.Logger.Warnf("Error retrieving read-only pad IDs: %v", err)
			return
		}
		p.SessionStore.addPadReadOnlyIds(client.SessionId, padIds.PadId, padIds.ReadOnlyPadId, padIds.ReadOnly)
		thisSession = p.SessionStore.getSession(client.SessionId)
	}

	var auth = thisSession.Auth

	if auth == nil {
		var ip string
		if ctx == nil {
			ip = "TEST"
		} else if settings.Displayed.DisableIPLogging {
			ip = "ANONYMOUS"
		} else {
			ip = ctx.IP()
		}
		println("pre-CLIENT_READY message from IP " + ip)
		return
	}

	var user *webaccess.SocketClientRequest
	if ctx != nil {
		var okConv bool
		user, okConv = ctx.Locals(clientVars2.WebAccessStore).(*webaccess.SocketClientRequest)
		if !okConv {
			user = nil
		}
	}

	var grantedAccess, err = p.securityManager.CheckAccess(&auth.PadId, &auth.SessionId, &auth.Token, user)

	if err != nil {
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		arr[1] = AccessStatusMessage{
			AccessStatus: err.Error(),
		}
		var messageToSend, _ = json.Marshal(arr)

		client.SafeSend(messageToSend)
		p.Logger.Warn("Error checking access", err.Error())
		return
	}

	if thisSession.Author != "" && thisSession.Author != grantedAccess.AuthorId {
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		arr[1] = UserDupMessage{
			Disconnect: "rejected",
		}
		var encoded, _ = json.Marshal(arr)
		client.SafeSend(encoded)
		return
	}

	thisSession.Author = grantedAccess.AuthorId

	var readonly = thisSession.ReadOnly
	var thisSessionNewRetrieved = p.SessionStore.getSession(client.SessionId)
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
	case ws.ChangesetReq:
		{
			p.HandleChangesetRequest(client, expectedType)
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
	case SavedRevision:
		{
			sess := p.SessionStore.getSession(client.SessionId)
			if sess == nil {
				p.Logger.Errorf("Session not found for saved revision")
				return
			}
			foundPad, err := p.padManager.GetPad(sess.PadId, nil, nil)
			if err != nil {
				p.Logger.Errorf("Error retrieving pad for saved revision: %v", err)
				return
			}
			p.HandleSavedRevisionMessage(foundPad, sess.Author)
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
			client.SafeSend(marshalled)
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

func (p *PadMessageHandler) HandleChangesetRequest(socket *Client, message ws.ChangesetReq) {
	if (message.Data.Data.Granularity <= 0) || (message.Data.Data.Start < 0) {
		println("Invalid changeset request parameters")
		return
	}
	start, err := utils.CheckValidRev(strconv.Itoa(message.Data.Data.Start))
	if err != nil {
		println("Error checking valid rev for changeset request", err)
		return
	}
	if message.Data.Data.RequestID == -1 {
		println("Invalid request ID for changeset request")
		return
	}

	startRev := *start

	end := startRev + (message.Data.Data.Granularity * 100)
	session := p.SessionStore.getSession(socket.SessionId)
	if session == nil {
		println("Session not found for changeset request")
		return
	}

	retrievedPad, err := p.padManager.GetPad(session.PadId, nil, &session.Author)
	if err != nil {
		println("Error retrieving pad for changeset request", err)
		return
	}
	headRev := retrievedPad.Head
	if startRev > headRev {
		startRev = headRev
	}

	data, err := p.getChangesetInfo(*retrievedPad, startRev, end, message.Data.Data.Granularity)
	if err != nil {
		println("Error getting changeset info for changeset request", err)
		return
	}

	var arr = make([]interface{}, 2)
	arr[0] = "message"
	data.RequestId = message.Data.Data.RequestID

	messageToSend := ChangesetResponse{
		Type: "CHANGESET_REQ",
		Data: *data,
	}

	arr[1] = messageToSend
	encoded, err := json.Marshal(arr)
	if err != nil {
		println("Error marshalling changeset response", err)
		return
	}

	socket.SafeSend(encoded)
}

func (p *PadMessageHandler) composePadChangesets(retrievedPad *pad2.Pad, start int, end int) (string, error) {
	// fetch all changesets we need
	var headNum = retrievedPad.Head
	endNum := math.Min(float64(end), float64(headNum+1))
	startNum := math.Max(float64(start), 0)

	var changesets = make([]string, 0)
	for i := int(startNum); i < int(endNum); i++ {
		nthChangeset, err := retrievedPad.GetRevisionChangeset(i)
		if err != nil {
			return "", fmt.Errorf("error retrieving changeset for revision %d: %v", i, err)
		}
		changesets = append(changesets, *nthChangeset)
	}

	startChangeset := changesets[0]
	for i := 1; i < len(changesets); i++ {
		startChangesetVar, err := changeset.Compose(startChangeset, changesets[i], &retrievedPad.Pool)
		if err != nil {
			return "", fmt.Errorf("error composing changesets: %v", err)
		}
		startChangeset = *startChangesetVar
	}
	return startChangeset, nil
}

type LineChange struct {
	Alines    []string
	TextLines []string
}

func getPadLines(retrievedPad *pad2.Pad, revNum int) (*LineChange, error) {
	var atext *apool.AText

	if revNum >= 0 {
		atext = retrievedPad.GetInternalRevisionAText(revNum)
	} else {
		replacementAText := changeset.MakeAText("\n", nil)
		atext = &replacementAText
	}

	if atext == nil {
		return nil, fmt.Errorf("could not retrieve atext for revision %d", revNum)
	}

	alines, err := changeset.SplitAttributionLines(atext.Attribs, atext.Text)
	if err != nil {
		return nil, fmt.Errorf("error splitting attribution lines: %v", err)
	}

	return &LineChange{
		TextLines: changeset.SplitTextLines(atext.Text),
		Alines:    alines,
	}, nil
}

func (p *PadMessageHandler) getChangesetInfo(retrievedPad pad2.Pad, startNum int, endNum int, granularity int) (*ChangesetInfo, error) {
	headRevision := retrievedPad.Head

	if endNum > headRevision+1 {
		endNum = headRevision + 1
	}
	endFloat := math.Floor(float64(endNum)/float64(granularity)) * float64(granularity)
	if endFloat >= math.MaxInt64 || endFloat <= math.MinInt64 {
		fmt.Println("f64 is out of int64 range.")
		return nil, errors.New("endNum value out of range")
	}

	endNum = int(endFloat)

	type CompositeChangesetInfo struct {
		Start int
		End   int
	}

	compositesChangesetNeeded := make([]CompositeChangesetInfo, 0)
	revTimesNeeded := make([]int64, 0)

	for start := startNum; start < endNum; start += granularity {
		endVar := start + granularity

		compositesChangesetNeeded = append(compositesChangesetNeeded, CompositeChangesetInfo{
			Start: start,
			End:   endVar,
		})

		// t1
		if start == 0 {
			revTimesNeeded = append(revTimesNeeded, 0)
		} else {
			revTimesNeeded = append(revTimesNeeded, int64(start-1))
		}

		// t2
		revTimesNeeded = append(revTimesNeeded, int64(endVar-1))
	}

	composedChangesets := make(map[string]string)

	revisionDate := make(map[int64]int64)

	lines, err := getPadLines(&retrievedPad, startNum-1)
	if err != nil {
		println("Error getting pad lines", err)
		return nil, err
	}

	for _, composeNeeded := range compositesChangesetNeeded {
		changesetComposed, err := p.composePadChangesets(&retrievedPad, composeNeeded.Start, composeNeeded.End)
		if err != nil {
			println("Error composing pad changesets", err)
			return nil, err
		}
		composedChangesets[fmt.Sprintf("%d/%d", composeNeeded.Start, composeNeeded.End)] = changesetComposed
	}

	for _, revTimeNeeded := range revTimesNeeded {
		revTime, _ := retrievedPad.GetRevisionDate(int(revTimeNeeded))
		revisionDate[revTimeNeeded] = *revTime
	}

	timeDeltas := make([]int64, 0)
	forwardChangesets := make([]string, 0)
	backwardChangesets := make([]string, 0)
	createdApool := apool.NewAPool()

	for compositeStart := startNum; compositeStart < endNum; compositeStart += granularity {
		compositeEnd := compositeStart + granularity
		if compositeEnd > endNum || compositeEnd > headRevision+1 {
			break
		}
		forwards := composedChangesets[fmt.Sprintf("%d/%d", compositeStart, compositeEnd)]
		backwards, err := changeset.Inverse(forwards, lines.TextLines, lines.Alines, &retrievedPad.Pool)
		if err != nil {
			println("Error getting inverse changeset", err)
			return nil, err
		}
		if err := changeset.MutateAttributionLines(forwards, &lines.Alines, &retrievedPad.Pool); err != nil {
			println("Error mutating attribution lines", err)
			return nil, err
		}
		if err := changeset.MutateTextLines(forwards, &lines.TextLines); err != nil {
			println("Error mutating text lines", err)
			return nil, err
		}

		forwards2 := changeset.MoveOpsToNewPool(forwards, &retrievedPad.Pool, &createdApool)
		backwards2 := changeset.MoveOpsToNewPool(*backwards, &retrievedPad.Pool, &createdApool)

		var t1 int64
		var t2 int64
		if compositeStart == 0 {
			t1 = revisionDate[0]
		} else {
			t1 = revisionDate[int64(compositeStart-1)]
		}

		t2 = revisionDate[int64(compositeEnd-1)]

		timeDeltas = append(timeDeltas, t2-t1)
		forwardChangesets = append(forwardChangesets, forwards2)
		backwardChangesets = append(backwardChangesets, backwards2)
	}

	return &ChangesetInfo{
		ForwardsChangesets:  forwardChangesets,
		BackwardsChangesets: backwardChangesets,
		APool:               createdApool.ToJsonable(),
		ActualEndNum:        endNum,
		TimeDeltas:          timeDeltas,
		Start:               startNum,
		Granularity:         granularity,
	}, nil
}

func (p *PadMessageHandler) SendChatMessageToPadClients(session *ws.Session, chatMessage ws.ChatMessageData) {
	var retrievedPad, err = p.padManager.GetPad(session.PadId, nil, chatMessage.AuthorId)
	if err != nil {
		println("Error retrieving pad for chat message", err)
		return
	}
	// pad.appendChatMessage() ignores the displayName property so we don't need to wait for
	// authorManager.getAuthorName() to resolve before saving the message to the database.
	_, err = retrievedPad.AppendChatMessage(chatMessage.AuthorId, *chatMessage.Time, chatMessage.Text)
	if err != nil {
		println("Error appending chat message to pad", err)
		return
	}
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

		socket.SafeSend(marshalledMessage)
	}
}

func (p *PadMessageHandler) HandlePadDelete(client *Client, padDeleteMessage PadDelete) {
	var session = p.SessionStore.getSession(client.SessionId)

	if session == nil || session.Author == "" || session.PadId == "" {
		println("Session not ready")
		return
	}

	retrievedPad, err := p.padManager.DoesPadExist(padDeleteMessage.Data.PadID)
	if err != nil {
		return
	}
	if !*retrievedPad {
		println("Pad does not exist")
		return
	}
	retrievedPadObj, err := p.padManager.GetPad(padDeleteMessage.Data.PadID, nil, nil)
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
	retrievedPad, err := p.padManager.DoesPadExist(padId)
	if err != nil {
		return err
	}
	if !*retrievedPad {
		return errors.New(constants.ErrorPadDoesNotExist)
	}
	retrievedPadObj, err := p.padManager.GetPad(padId, nil, nil)
	if err != nil {
		return err
	}
	if err := retrievedPadObj.Remove(); err != nil {
		return err
	}
	p.KickSessionsFromPad(retrievedPadObj.Id)
	// remove the readonly entries
	var readonlyId = p.readOnlyManager.GetReadOnlyId(retrievedPadObj.Id)
	err = p.readOnlyManager.RemoveReadOnlyPad(readonlyId, retrievedPadObj.Id)
	if err != nil {
		return err
	}
	if err := retrievedPadObj.RemoveAllChats(); err != nil {
		return err
	}

	if err := retrievedPadObj.RemoveAllSavedRevisions(); err != nil {
		return err
	}
	if err := p.padManager.RemovePad(retrievedPadObj.Id); err != nil {
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
	var session = p.SessionStore.getSession(client.SessionId)

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
		p.SafeSend(marshalled)
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
		builder.KeepText(text[offset:i], nil, nil)
		builder.Remove(1, 0)
		offset = i + 1
	}

	var stringifierBuilder = builder.ToString()
	return &stringifierBuilder
}

func (p *PadMessageHandler) HandleDisconnectOfPadClient(client *Client, settings *settings.Settings, logger *zap.SugaredLogger) {
	var thisSession = p.SessionStore.getSession(client.SessionId)
	if thisSession == nil || thisSession.PadId == "" {
		p.SessionStore.removeSession(client.SessionId)
		return
	}

	if settings.DisableIPLogging {
		logger.Infof("[LEAVE] pad:%s socket:%s IP:ANONYMOUS ", thisSession.PadId, client.SessionId)
	} else {
		logger.Infof("[LEAVE] pad:%s socket:%s IP:%s ", thisSession.PadId, client.SessionId, client.Ctx.IP())
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
		otherSocket.SafeSend(marshalled)
	}

	p.SessionStore.removeSession(client.SessionId)
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
		argsForLogger = append(argsForLogger, client.Ctx.IP())
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
	chatAuthors, err := retrievedPad.GetAllChatters()
	if err != nil {
		p.Logger.Errorf("Error retrieving chat authors")
		return
	}

	setOfAuthors := make(map[string]struct{})
	for _, a := range authors {
		setOfAuthors[a] = struct{}{}
	}
	for _, ca := range *chatAuthors {
		setOfAuthors[ca] = struct{}{}
	}

	var _ = retrievedPad.GetPadMetaData(retrievedPad.Head)

	var historicalAuthorData = make(map[string]author.Author)

	for a := range setOfAuthors {
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
		var sinfo = p.SessionStore.getSession(otherSocket.SessionId)
		if sinfo == nil {
			continue
		}

		if sinfo.Author == thisSession.Author {
			p.SessionStore.resetSession(otherSocket.SessionId)
			otherSocket.Leave()
			var arr = make([]interface{}, 2)
			arr[0] = "message"
			arr[1] = UserDupMessage{
				Disconnect: "userdup",
			}
			var encoded, _ = json.Marshal(arr)
			otherSocket.SafeSend(encoded)
		}
	}

	if ready.Data.Reconnect != nil && *ready.Data.Reconnect {
		// This is a reconnect - the client already has the pad content
		// We need to send any changesets the client missed while disconnected
		thisSession.PadId = retrievedPad.Id

		// Get the client's current revision
		clientRev := 0
		if ready.Data.ClientRev != nil {
			clientRev = *ready.Data.ClientRev
		}

		// Save the revision in sessioninfos
		thisSession.Revision = clientRev

		headRev := retrievedPad.Head

		// Calculate the range of revisions needed
		startNum := clientRev + 1
		endNum := headRev + 1

		if endNum > headRev+1 {
			endNum = headRev + 1
		}
		if startNum < 0 {
			startNum = 0
		}

		logger.Infof("Client reconnected to pad %s at revision %d (head: %d), sending revisions %d to %d",
			thisSession.PadId, clientRev, headRev, startNum, endNum-1)

		// If there are revisions to send
		if startNum < endNum {
			// Load all needed revisions in one query
			revisions, err := retrievedPad.GetRevisions(startNum, endNum-1)
			if err != nil {
				logger.Warnf("Error getting revisions for reconnect: %v", err)
				return
			}

			// Send each missed revision as CLIENT_RECONNECT
			for _, rev := range *revisions {
				forWire := changeset.PrepareForWire(rev.Changeset, retrievedPad.Pool)
				wirePool := forWire.Pool.ToJsonable()

				var authorId string
				if rev.AuthorId != nil {
					authorId = *rev.AuthorId
				}

				reconnectMsg := ClientReconnectMessage{
					Type: "COLLABROOM",
					Data: ClientReconnectData{
						Type:        "CLIENT_RECONNECT",
						HeadRev:     headRev,
						NewRev:      rev.RevNum,
						Changeset:   forWire.Translated,
						APool:       wirePool,
						Author:      authorId,
						CurrentTime: rev.Timestamp,
					},
				}

				arr := make([]interface{}, 2)
				arr[0] = "message"
				arr[1] = reconnectMsg

				encoded, err := json.Marshal(arr)
				if err != nil {
					logger.Warnf("Error marshaling CLIENT_RECONNECT message: %v", err)
					continue
				}

				client.SafeSend(encoded)

				// Update session revision
				thisSession.Revision = rev.RevNum
				thisSession.Time = rev.Timestamp
			}
		} else {
			// No changes - send a noChanges message
			noChangesMsg := ClientReconnectMessage{
				Type: "COLLABROOM",
				Data: ClientReconnectData{
					Type:      "CLIENT_RECONNECT",
					NoChanges: true,
					NewRev:    headRev,
				},
			}

			arr := make([]interface{}, 2)
			arr[0] = "message"
			arr[1] = noChangesMsg

			encoded, _ := json.Marshal(arr)
			client.SafeSend(encoded)

			thisSession.Revision = headRev
		}
	} else {
		var atext = changeset.CloneAText(retrievedPad.AText)
		var attribsForWire = changeset.PrepareForWire(atext.Attribs, retrievedPad.Pool)
		atext.Attribs = attribsForWire.Translated
		wirePool := attribsForWire.Pool.ToJsonable()
		retrivedClientVars, err := p.factory.NewClientVars(*retrievedPad, thisSession, wirePool, atext.Attribs, historicalAuthorData, retrievedSettings)
		if err != nil {
			println("Error creating client vars", err.Error())
			return
		}
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		arr[1] = Message{
			Data: *retrivedClientVars,
			Type: "CLIENT_VARS",
		}
		var encoded, _ = json.Marshal(arr)
		// Join the pad and start receiving updates
		thisSession.PadId = retrievedPad.Id
		// Send the clientVars to the Client
		client.SafeSend(encoded)
		// Save the current revision in sessioninfos, should be the same as in clientVars
		thisSession.Revision = retrievedPad.Head
	}

	retrievedAuthor, err := p.authorManager.GetAuthor(thisSession.Author)
	if err != nil {
		println("Error retrieving author for USER_NEWINFO broadcast")
		return
	}

	// Create and broadcast USER_NEWINFO message to all Clients in the pad
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
		socket.SafeSend(marshalled)
	}

	// send all other users' info to the new client
	for _, socket := range roomSockets {
		if socket.SessionId == client.SessionId {
			continue
		}
		var sinfo = p.SessionStore.getSession(socket.SessionId)
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

		client.SafeSend(marshalled)
	}
}

func (p *PadMessageHandler) UpdatePadClients(pad *pad2.Pad) {
	var roomSockets = p.GetRoomSockets(pad.Id)
	if len(roomSockets) == 0 {
		return
	}
	// since all Clients usually get the same set of changesets, store them in local cache
	// to remove unnecessary roundtrip to the datalayer
	// NB: note below possibly now accommodated via the change to promises/async
	var revCache = make(map[int]*db.PadSingleRevision)
	for _, socket := range roomSockets {
		var sessionInfo = p.SessionStore.getSession(socket.SessionId)
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

			socket.SafeSend(marshalledMessage)
			sessionInfo.Time = currentTime
			sessionInfo.Revision = r
		}
	}
}

func (p *PadMessageHandler) GetRoomSockets(padID string) []Client {
	var sockets = make([]Client, 0)
	p.hub.ClientsRWMutex.RLock()
	for k := range p.hub.Clients {
		sessId := p.SessionStore.getSession(k.SessionId)
		if sessId != nil && sessId.PadId == padID {
			sockets = append(sockets, *k)
		}
	}
	p.hub.ClientsRWMutex.RUnlock()
	return sockets
}

func (p *PadMessageHandler) KickSessionsFromPad(padID string) {
	p.hub.ClientsRWMutex.RLock()
	for k := range p.hub.Clients {
		if k == nil || k.SessionId == "" {
			continue
		}
		retrievedSession := p.SessionStore.getSession(k.SessionId)
		if retrievedSession == nil {
			continue
		}

		if retrievedSession.PadId == padID {
			k.SendPadDelete()
		}
	}
	p.hub.ClientsRWMutex.RUnlock()
}

func (p *PadMessageHandler) HandleSavedRevisionMessage(foundPad *pad2.Pad, author string) {
	if err := foundPad.AddSavedRevision(author); err != nil {
		p.Logger.Warnf("Error adding saved revision:%s", err)
	}
	p.Logger.Infof("Added saved revision:%v by %s", foundPad.Id, author)
}
