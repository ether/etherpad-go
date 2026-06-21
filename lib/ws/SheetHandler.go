package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/ether/etherpad-go/lib/sheetdoc"
)

// SheetTask is one queued SHEET_OP awaiting serialized processing per document.
type SheetTask struct {
	socket  *Client
	message ws.SheetOpIncoming
}

// SheetChannelOperator serializes SHEET_OPs per sheet document via one goroutine
// per pad id, mirroring ChannelOperator for the text pad.
type SheetChannelOperator struct {
	channels map[string]chan SheetTask
	handler  *PadMessageHandler
	mu       sync.Mutex
}

func NewSheetChannelOperator(p *PadMessageHandler) SheetChannelOperator {
	return SheetChannelOperator{
		channels: make(map[string]chan SheetTask),
		handler:  p,
	}
}

func (c *SheetChannelOperator) AddToQueue(ch string, t SheetTask) {
	c.mu.Lock()
	chChan, ok := c.channels[ch]
	if !ok {
		chChan = make(chan SheetTask, 1)
		c.channels[ch] = chChan
		go func(localCh chan SheetTask) {
			for incomingTask := range localCh {
				c.handler.handleSheetOp(incomingTask)
			}
		}(chChan)
	}
	c.mu.Unlock()
	chChan <- t
}

// SheetManager exposes the shared sheet document manager so HTTP handlers
// (xlsx import/export) operate on the same live state as the websocket clients.
func (p *PadMessageHandler) SheetManager() *sheetdoc.Manager {
	return p.sheetManager
}

// BroadcastSheetReload tells every client of a sheet to re-fetch its state
// (used after an xlsx import replaces the workbook).
func (p *PadMessageHandler) BroadcastSheetReload(padId string) {
	encoded, err := json.Marshal([]any{"message", map[string]any{
		"type": "COLLABROOM",
		"data": map[string]any{"type": "SHEET_RELOAD"},
	}})
	if err != nil {
		return
	}
	for _, socket := range p.GetRoomSockets(padId) {
		socket.SafeSend(encoded)
	}
}

// EnqueueSheetOp routes a SHEET_OP to the per-document serialization goroutine.
// Keyed by the session's pad id so each document keeps a total order.
func (p *PadMessageHandler) EnqueueSheetOp(client *Client, msg ws.SheetOpIncoming) {
	session := p.SessionStore.getSession(client.SessionId)
	if session == nil || session.PadId == "" {
		p.Logger.Warn("SHEET_OP before session ready")
		return
	}
	p.sheetChannels.AddToQueue(session.PadId, SheetTask{socket: client, message: msg})
}

// handleSheetOp applies one op via the sheet document manager, acks the sender,
// and broadcasts the rebased op to the other clients of the document.
func (p *PadMessageHandler) handleSheetOp(task SheetTask) {
	session := p.SessionStore.getSession(task.socket.SessionId)
	if session == nil || session.PadId == "" {
		return
	}
	if session.ReadOnly {
		p.Logger.Warn("write attempt on read-only sheet")
		return
	}

	var op sheet.Op
	if err := json.Unmarshal(task.message.Data.Data.Op, &op); err != nil {
		p.Logger.Warn("bad sheet op: ", err)
		return
	}
	op.BaseRev = task.message.Data.Data.BaseRev

	author := session.Author
	rebased, newRev, err := p.sheetManager.Submit(session.PadId, op, &author, time.Now().UnixMilli())
	if err != nil {
		p.Logger.Warn("sheet submit failed: ", err)
		return
	}

	p.sendAcceptSheetOp(task.socket, newRev)
	p.broadcastNewSheetOp(session.PadId, task.socket.SessionId, rebased, newRev, author)
}

func (p *PadMessageHandler) sendAcceptSheetOp(client *Client, newRev int) {
	msg := ws.AcceptSheetOp{Type: "COLLABROOM"}
	msg.Data.Type = "ACCEPT_SHEET_OP"
	msg.Data.NewRev = newRev
	encoded, err := json.Marshal([]any{"message", msg})
	if err != nil {
		p.Logger.Warn("marshal ACCEPT_SHEET_OP: ", err)
		return
	}
	client.SafeSend(encoded)
}

func (p *PadMessageHandler) broadcastNewSheetOp(padId string, senderSessionId string, rebased sheet.Op, newRev int, author string) {
	opBytes, err := json.Marshal(rebased)
	if err != nil {
		p.Logger.Warn("marshal rebased sheet op: ", err)
		return
	}
	msg := ws.NewSheetOp{Type: "COLLABROOM"}
	msg.Data.Type = "NEW_SHEET_OP"
	msg.Data.Op = opBytes
	msg.Data.NewRev = newRev
	msg.Data.Author = author
	encoded, err := json.Marshal([]any{"message", msg})
	if err != nil {
		p.Logger.Warn("marshal NEW_SHEET_OP: ", err)
		return
	}
	for _, socket := range p.GetRoomSockets(padId) {
		if socket.SessionId == senderSessionId {
			continue
		}
		socket.SafeSend(encoded)
	}
}

// HandleSheetClientReady materializes the pad as a sheet, then either sends the
// full SHEET_VARS snapshot (fresh connect) or the missed ops (reconnect), and
// announces presence to the other clients of the document.
func (p *PadMessageHandler) HandleSheetClientReady(ready ws.ClientReady, client *Client, session *ws.Session) {
	if _, err := p.padManager.GetTypedPad(session.PadId, "sheet", &session.Author); err != nil {
		p.Logger.Warn("error materializing sheet pad: ", err)
		return
	}

	if ready.Data.Reconnect != nil && *ready.Data.Reconnect {
		clientRev := 0
		if ready.Data.ClientRev != nil {
			clientRev = *ready.Data.ClientRev
		}
		ops, err := p.sheetManager.OpsSince(session.PadId, clientRev)
		if err != nil {
			p.Logger.Warn("OpsSince failed: ", err)
			return
		}
		for i, op := range ops {
			p.sendReconnectSheetOp(client, op, clientRev+i+1)
		}
	} else {
		p.sendSheetVars(client, session)
	}

	p.announceSheetPresence(client, session)
}

func (p *PadMessageHandler) sendSheetVars(client *Client, session *ws.Session) {
	snap, head, err := p.sheetManager.Snapshot(session.PadId)
	if err != nil {
		p.Logger.Warn("sheet snapshot failed: ", err)
		return
	}
	snapBytes, err := json.Marshal(snap)
	if err != nil {
		p.Logger.Warn("marshal snapshot: ", err)
		return
	}

	var color string
	if a, err := p.authorManager.GetAuthor(session.Author); err == nil && a != nil {
		color = a.ColorId
	}

	sv := ws.SheetVars{Type: "SHEET_VARS", Data: ws.SheetVarsData{
		Snapshot:  snapBytes,
		Head:      head,
		UserId:    session.Author,
		UserColor: color,
		ReadOnly:  session.ReadOnly,
	}}
	encoded, err := json.Marshal([]any{"message", sv})
	if err != nil {
		p.Logger.Warn("marshal SHEET_VARS: ", err)
		return
	}
	client.SafeSend(encoded)
}

func (p *PadMessageHandler) sendReconnectSheetOp(client *Client, op sheet.Op, rev int) {
	opBytes, err := json.Marshal(op)
	if err != nil {
		return
	}
	msg := ws.NewSheetOp{Type: "COLLABROOM"}
	msg.Data.Type = "NEW_SHEET_OP"
	msg.Data.Op = opBytes
	msg.Data.NewRev = rev
	encoded, err := json.Marshal([]any{"message", msg})
	if err != nil {
		return
	}
	client.SafeSend(encoded)
}

// announceSheetPresence broadcasts the joining user's info to the other clients
// of the document, reusing the text pad's USER_NEWINFO frame.
func (p *PadMessageHandler) announceSheetPresence(client *Client, session *ws.Session) {
	a, err := p.authorManager.GetAuthor(session.Author)
	if err != nil || a == nil {
		return
	}
	info := ws.UserNewInfo{Type: "COLLABROOM", Data: ws.UserNewInfoData{
		Type: "USER_NEWINFO",
		UserInfo: ws.UserNewInfoDat{
			UserId:  session.Author,
			Name:    a.Name,
			ColorId: a.ColorId,
		},
	}}
	encoded, err := json.Marshal([]any{"message", info})
	if err != nil {
		return
	}
	for _, socket := range p.GetRoomSockets(session.PadId) {
		if socket.SessionId == client.SessionId {
			continue
		}
		socket.SafeSend(encoded)
	}
}
