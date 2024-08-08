package ws

import (
	"encoding/json"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings/clientVars"
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

func init() {
	padManager = pad.NewManager()
	readOnlyManager = pad.NewReadOnlyManager()
	authorManager = author.NewManager()
	colorRegEx, _ = regexp.Compile("^#(?:[0-9A-F]{3}){1,2}$")
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
		NextNum:        task.message.Data.Apool.NextNum,
		NumToAttribRaw: task.message.Data.Apool.NumToAttrib,
	})
	var session = SessionStore[task.socket.SessionId]

	var retrievedPad, _ = padManager.GetPad(session.PadId, nil, &session.Author)
	_, err := changeset.CheckRep(task.message.Data.Changeset)

	if err != nil {
		return
	}

	unpackedChangeset, err := changeset.Unpack(task.message.Data.Changeset)

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

		if opAuthorId != "" && opAuthorId != session.Author {
			println("Wrong author tried to submit changeset")
		}
	}

	rebasedChangeset := changeset.MoveOpsToNewPool(task.message.Data.Changeset, *wireApool, retrievedPad.Pool)

	var r = task.message.Data.BaseRev

	for r < retrievedPad.Head {
		r++
		var revisionPad, _ = retrievedPad.GetRevision(r)

		if revisionPad.Changeset == task.message.Data.Changeset && revisionPad.AuthorId == &session.Author {
			// Assume this is a retransmission of an already applied changeset.
			unpackedChangeset, _ = changeset.Unpack(task.message.Data.Changeset)
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

	if session.revision != r {
		println("Revision mismatch")
	}

	// TODO hier weiterschreiben

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

func HandleClientReadyMessage(ready ws.ClientReady, client *Client) {

	var sessionInfo = SessionStore[client.SessionId]
	var authSession = AuthSession{
		PadID: ready.Data.PadID,
		Token: ready.Data.Token,
	}

	if !padManager.DoesPadExist(ready.Data.PadID) {
		var padId, err = padManager.SanitizePadId(ready.Data.PadID)

		if err != nil {
			println("Error sanitizing pad id", err.Error())
			return
		}
		authSession.PadID = *padId
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

	var retrievedPad, err = padManager.GetPad(authSession.PadID, nil, &foundAuthor.Id)
	if err != nil {
		println("Error getting pad")
		return
	}

	var authors = retrievedPad.GetAllAuthors()

	var _ = retrievedPad.GetPadMetaData(retrievedPad.Head)

	var historicalAuthorData = make(map[string]author.Author)

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

	if ready.Data.Reconnect != nil && *ready.Data.Reconnect {

	} else {
		var atext = changeset.CloneAText(retrievedPad.AText)
		var attribsForWire = changeset.PrepareForWire(atext.Attribs, retrievedPad.Pool)
		atext.Attribs = attribsForWire.Translated
		var arr = make([]interface{}, 2)
		arr[0] = "message"
		arr[1] = Message{
			Data: clientVars.NewClientVars(*retrievedPad),
			Type: "CLIENT_VARS",
		}
		var encoded, _ = json.Marshal(arr)
		client.conn.WriteMessage(websocket.TextMessage, encoded)

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
