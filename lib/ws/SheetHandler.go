package ws

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/cluster"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/sheet"
	"github.com/ether/etherpad-go/lib/sheetdoc"
)

// SheetTask is one queued SHEET_OP awaiting serialized processing per document.
type SheetTask struct {
	socket  *Client
	message ws.SheetOpIncoming
}

// SheetChannelOperator serializes SHEET_OPs per sheet document. Like
// ChannelOperator it runs each op on the sender's read loop under the
// document's write lock instead of keeping a goroutine per document alive
// forever; apply and broadcast stay in one total order per document.
type SheetChannelOperator struct {
	handler *PadMessageHandler
}

func NewSheetChannelOperator(p *PadMessageHandler) SheetChannelOperator {
	return SheetChannelOperator{handler: p}
}

func (c *SheetChannelOperator) AddToQueue(ch string, t SheetTask) {
	unlock, err := c.handler.padManager.LockPad(ch)
	if err != nil {
		// Not applied: the client gets no acknowledgement and resends.
		c.handler.Logger.Warnw("sheet op not applied: pad unavailable", "pad", ch, "error", err)
		return
	}
	defer unlock()
	c.handler.handleSheetOp(t)
}

// SheetManager exposes the shared sheet document manager so HTTP handlers
// (xlsx import/export) operate on the same live state as the websocket clients.
func (p *PadMessageHandler) SheetManager() *sheetdoc.Manager {
	return p.sheetManager
}

// BroadcastSheetReload tells every client of a sheet to re-fetch its state
// (used after an xlsx import replaces the workbook). The caller holds the
// pad's write lock. Clients on other instances are told by their instance,
// which notices the new epoch.
func (p *PadMessageHandler) BroadcastSheetReload(padId string) {
	p.flushSheet(padId, "", nil, 0)
	p.publish(cluster.Event{Kind: cluster.KindRevision, Pad: padId})
}

// sheetProgress is what this instance's clients of a sheet have been sent.
type sheetProgress struct {
	epoch int
	rev   int
}

// sheetState tracks sheetProgress per sheet.
type sheetState struct {
	mu   sync.Mutex
	sent map[string]sheetProgress
}

func (p *PadMessageHandler) sendSheetReload(sockets []Client, except string) {
	encoded, err := json.Marshal([]any{"message", map[string]any{
		"type": "COLLABROOM",
		"data": map[string]any{"type": "SHEET_RELOAD"},
	}})
	if err != nil {
		return
	}
	for _, socket := range sockets {
		if socket.SessionId != except {
			socket.SafeSend(encoded)
		}
	}
}

// flushSheet sends this instance's clients of a sheet what they have not
// been sent yet: the ops stored since the last flush, in revision order,
// whichever instance stored them, or a reload if the workbook was replaced.
// Clients apply ops in the order they arrive, so every op reaches them
// through this one path. sender, if set, gets ACCEPT_SHEET_OP for acceptRev
// in place of that op; the session except gets nothing (it is joining and
// gets its state separately). The caller holds the pad's local write lock.
func (p *PadMessageHandler) flushSheet(padId string, except string, sender *Client, acceptRev int) {
	epoch, head, err := p.sheetManager.State(padId)
	if err != nil {
		p.Logger.Warn("sheet state unavailable: ", err)
		return
	}
	p.sheets.mu.Lock()
	defer p.sheets.mu.Unlock()
	if p.sheets.sent == nil {
		p.sheets.sent = map[string]sheetProgress{}
	}
	st, known := p.sheets.sent[padId]
	if !known {
		// Nothing was sent for this sheet here yet: the clients are at the
		// head, or at the state before the op being acknowledged.
		st = sheetProgress{epoch: epoch, rev: head}
		if sender != nil {
			st.rev = acceptRev - 1
		}
	}
	sockets := p.GetRoomSockets(padId)
	switch {
	case st.epoch != epoch || head < st.rev:
		p.sendSheetReload(sockets, except)
	case head > st.rev:
		ops, err := p.sheetManager.StoredOps(padId, st.rev+1, head)
		if err != nil {
			// Left for the next flush (the periodic catch-up at the latest).
			p.Logger.Warn("sheet ops unavailable: ", err)
			return
		}
		for _, op := range ops {
			encoded, ok := p.newSheetOpMessage(op.Op, op.Rev, op.Author)
			if !ok {
				continue
			}
			for _, socket := range sockets {
				switch {
				case socket.SessionId == except:
				case sender != nil && socket.SessionId == sender.SessionId && op.Rev == acceptRev:
					p.sendAcceptSheetOp(sender, acceptRev)
				default:
					socket.SafeSend(encoded)
				}
			}
		}
	}
	p.sheets.sent[padId] = sheetProgress{epoch: epoch, rev: head}
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
	_, newRev, err := p.sheetManager.Submit(session.PadId, op, &author, time.Now().UnixMilli())
	if err != nil {
		p.Logger.Warn("sheet submit failed: ", err)
		return
	}

	// Sends the clients here everything up to this op in order (also ops
	// other instances stored in between) and the sender its acknowledgement.
	p.flushSheet(session.PadId, "", task.socket, newRev)
	// Tell the other instances; they send their clients the op from the
	// database.
	p.publish(cluster.Event{Kind: cluster.KindRevision, Pad: session.PadId, Rev: newRev})
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

// newSheetOpMessage encodes a NEW_SHEET_OP message.
func (p *PadMessageHandler) newSheetOpMessage(op sheet.Op, rev int, author string) ([]byte, bool) {
	opBytes, err := json.Marshal(op)
	if err != nil {
		p.Logger.Warn("marshal sheet op: ", err)
		return nil, false
	}
	msg := ws.NewSheetOp{Type: "COLLABROOM"}
	msg.Data.Type = "NEW_SHEET_OP"
	msg.Data.Op = opBytes
	msg.Data.NewRev = rev
	msg.Data.Author = author
	encoded, err := json.Marshal([]any{"message", msg})
	if err != nil {
		p.Logger.Warn("marshal NEW_SHEET_OP: ", err)
		return nil, false
	}
	return encoded, true
}

// HandlePresence relays an ephemeral cursor / live-edit frame to the other
// clients of the sheet. It is NOT persisted and NOT ordered through the per-doc
// goroutine. Identity is stamped server-side from the session author (no client
// spoofing); read-only sessions may move a cursor but never broadcast a live
// edit.
func (p *PadMessageHandler) HandlePresence(client *Client, msg ws.SheetPresenceIncoming) {
	session := p.SessionStore.getSession(client.SessionId)
	if session == nil || session.PadId == "" {
		return
	}

	editing := msg.Data.Data.Editing
	raw := msg.Data.Data.Raw
	if session.ReadOnly {
		editing = false
		raw = ""
	}

	var name, color string
	if a, err := p.authorManager.GetAuthor(session.Author); err == nil && a != nil {
		if a.Name != nil {
			name = *a.Name
		}
		color = a.ColorId
	} else if err != nil {
		p.Logger.Warn("SHEET_PRESENCE: author lookup failed for ", session.Author, ": ", err)
		// relay continues with empty name/color — presence is best-effort
	}

	out := ws.SheetPresence{Type: "COLLABROOM"}
	out.Data = ws.SheetPresenceData{
		Type:     "SHEET_PRESENCE",
		UserId:   session.Author,
		Name:     name,
		Color:    color,
		Sheet:    msg.Data.Data.Sheet,
		Row:      msg.Data.Data.Row,
		Col:      msg.Data.Data.Col,
		Editing:  editing,
		Raw:      raw,
		FocusRow: msg.Data.Data.FocusRow,
		FocusCol: msg.Data.Data.FocusCol,
	}
	encoded, err := json.Marshal([]any{"message", out})
	if err != nil {
		p.Logger.Warn("marshal SHEET_PRESENCE: ", err)
		return
	}
	p.broadcastToRoom(session.PadId, encoded, client.SessionId)
}

// HandleSheetClientReady materializes the pad as a sheet, then either sends the
// full SHEET_VARS snapshot (fresh connect) or the missed ops (reconnect), and
// announces presence to the other clients of the document.
func (p *PadMessageHandler) HandleSheetClientReady(ready ws.ClientReady, client *Client, session *ws.Session) {
	// The state sent to the joining client and the ops sent to the others
	// must not interleave with an op being applied.
	unlock, err := p.padManager.LockPad(session.PadId)
	if err != nil {
		p.dropUnavailable(client, session.PadId, err)
		return
	}
	defer unlock()
	if _, err := p.padManager.GetTypedPad(session.PadId, "sheet", &session.Author); err != nil {
		p.Logger.Warn("error materializing sheet pad: ", err)
		return
	}
	// Bring the clients already here up to the head the joining client
	// starts from.
	p.flushSheet(session.PadId, client.SessionId, nil, 0)

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
	p.broadcastToRoom(session.PadId, encoded, client.SessionId)
}
