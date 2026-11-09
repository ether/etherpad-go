package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"unicode/utf8"

	"flag"
	"io"
	"time"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

type Pad struct {
	logger    *zap.SugaredLogger
	host      string
	padId     string
	apool     *apool.APool
	baseRev   int
	atext     *apool.AText
	conn      *websocket.Conn
	connWrite sync.Mutex
	poolLock  sync.RWMutex
	events    map[string][]func(interface{})
	closeChan chan struct{}
	closeOnce sync.Once
	inFlight  *PadChangeset
	outgoing  *PadChangeset
	// authorId is the author the server assigned in CLIENT_VARS. Inserts
	// must carry it as author attribute, or the server rejects them.
	authorId string
	// token identifies the author; a reconnect sends the same one.
	token string
	path  string
	// reconnectHosts, if set, are tried when the connection breaks.
	reconnectHosts []string
	// reconnecting is true from a broken connection until the missed
	// revisions are applied; no change is sent meanwhile.
	reconnecting bool
}

func NewPad(host, padId string, conn *websocket.Conn, logger *zap.SugaredLogger) *Pad {
	return &Pad{
		logger:    logger,
		host:      host,
		padId:     padId,
		conn:      conn,
		connWrite: sync.Mutex{},
		events:    make(map[string][]func(interface{})),
		closeChan: make(chan struct{}),
	}
}

func (p *Pad) On(event string, handler func(interface{})) {
	p.events[event] = append(p.events[event], handler)
}

func (p *Pad) emit(event string, data interface{}) {
	for _, handler := range p.events[event] {
		go handler(data)
	}
}

func (p *Pad) Close() {
	p.closeOnce.Do(func() {
		close(p.closeChan)
		if p.conn != nil {
			_ = p.conn.Close()
		}
		p.emit("disconnect", nil)
	})
}

func (p *Pad) Append(text string) {
	// Acquire lock while we read/modify shared pad state (atext, apool, baseRev)
	p.poolLock.Lock()
	if p.atext == nil || p.apool == nil {
		p.poolLock.Unlock()
		p.logger.Errorf("Pad is not initialized")
		return
	}

	if len(text) == 0 {
		p.logger.Warnf("No text to append - changeset will not be created.")
		p.poolLock.Unlock()
		return
	}

	if text == "\n" && strings.HasSuffix(p.atext.Text, "\n") {
		p.logger.Infof("Pad already ends with newline - not appending another.")
		p.poolLock.Unlock()
		return
	}

	if text[len(text)-1] != '\n' {
		text += "\n"
	}

	start := utf8.RuneCountInString(p.atext.Text)
	attribs := ""
	if p.authorId != "" {
		num := p.apool.PutAttrib(apool.Attribute{Key: "author", Value: p.authorId}, nil)
		attribs = "*" + utils.NumToString(num)
	}
	newChangeset, err := changeset.MakeSplice(p.atext.Text, start, 0, text, &attribs, p.apool)
	if err != nil {
		p.poolLock.Unlock()
		p.logger.Errorf("Error creating changeset: %v", err)
		return
	}

	// Unpack and repack to ensure canonical form
	unpacked, err := changeset.Unpack(newChangeset)
	if err != nil {
		p.poolLock.Unlock()
		p.logger.Errorf("Error unpacking changeset: %v", err)
		return
	}
	newChangeset = changeset.Pack(unpacked.OldLen, unpacked.NewLen, unpacked.Ops, unpacked.CharBank)

	// Validate generated changeset header: oldLen should equal current local text length
	if unpacked.OldLen != start {
		p.poolLock.Unlock()
		p.logger.Errorf("Generated changeset oldLen mismatch: expected %d got %d; not sending", start, unpacked.OldLen)
		// emit an error event so callers/tests can react
		p.emit("append_error", map[string]interface{}{"error": "oldLen_mismatch", "expected": start, "got": unpacked.OldLen})
		return
	}

	newAText, err := changeset.ApplyToAText(newChangeset, *p.atext, *p.apool)
	if err != nil {
		p.poolLock.Unlock()
		p.logger.Errorf("Error applying changeset: %v", err)
		return
	}

	p.atext = newAText
	// Queue the changeset for sending; inFlight/outgoing are guarded by
	// poolLock like the text they belong to.
	p.sendMessage(&PadChangeset{changeset: newChangeset, baseRev: p.baseRev})
	p.poolLock.Unlock()
}

type PadState struct {
	Host  string
	Path  string
	PadId string
}

func Connect(host string, logger *zap.SugaredLogger) *Pad {
	return connect(host, logger)
}

func connect(host string, logger *zap.SugaredLogger) *Pad {
	padState := parsePadURL(host, logger)

	httpClient := &http.Client{}
	fullUrl := fmt.Sprintf("%s%s/p/%s", padState.Host, padState.Path, padState.PadId)
	logger.Infof("Getting Pad at %s", fullUrl)
	resp, err := httpClient.Get(fullUrl)
	if err != nil {
		logger.Errorf("Failed to connect to pad at %s: %v", fullUrl, err)
		os.Exit(1)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		logger.Errorf("Failed to connect to pad at %s, status: %s, body: %s", fullUrl, resp.Status, string(body))
		os.Exit(1)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	connection, err := dialPad(padState, logger)
	if err != nil {
		os.Exit(1)
	}

	pad := NewPad(padState.Host, padState.PadId, connection, logger)
	pad.token = "t." + utils.RandomString(20)
	pad.path = padState.Path
	go pad.readLoop(connection)
	pad.sendClientReady(connection, false)
	return pad
}

func parsePadURL(host string, logger *zap.SugaredLogger) PadState {
	padState := PadState{}
	if host == "" {
		padState.Host = "http://127.0.0.1:9001"
		padState.Path = "/p/test"
		padState.PadId = utils.RandomString(10)
		return padState
	}
	parsedUrl, err := url.Parse(host)
	if err != nil {
		logger.Warnf("Invalid host URL: %v", err)
		os.Exit(1)
	}
	padState.Host = fmt.Sprintf("%s://%s", parsedUrl.Scheme, parsedUrl.Host)
	const padIdParam = "/p/"
	indexOfPadId := strings.Index(parsedUrl.Path, padIdParam)
	if indexOfPadId == -1 {
		padState.Path = ""
		padState.PadId = utils.RandomString(10)
	} else {
		padState.Path = parsedUrl.Path[0:indexOfPadId]
		padState.PadId = parsedUrl.Path[indexOfPadId+len(padIdParam):]
	}
	return padState
}

func dialPad(padState PadState, logger *zap.SugaredLogger) (*websocket.Conn, error) {
	wsUrl := fmt.Sprintf("%s/%ssocket.io", strings.Replace(padState.Host, "http", "ws", 1), padState.Path)
	logger.Infof("Connecting to WebSocket at %s", wsUrl)
	connection, resp, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		logger.Errorf("WebSocket connection failed: %v", err)
		if resp != nil {
			logger.Warnf("Response Status: %s", resp.Status)
		}
		return nil, err
	}
	return connection, nil
}

// sendClientReady joins the pad. On a reconnect it tells the server the
// last revision this client has, like the browser does; the server answers
// with the missed revisions as CLIENT_RECONNECT.
func (p *Pad) sendClientReady(conn *websocket.Conn, reconnect bool) {
	data := ws.ClientReadyData{
		Component: "pad",
		Type:      "CLIENT_READY",
		PadID:     p.padId,
		Token:     p.token,
		UserInfo:  ws.ClientReadyUserInfo{},
	}
	if reconnect {
		p.poolLock.RLock()
		rev := p.baseRev
		p.poolLock.RUnlock()
		yes := true
		data.Reconnect = &yes
		data.ClientRev = &rev
	}
	p.connWrite.Lock()
	defer p.connWrite.Unlock()
	if err := conn.WriteJSON(ws.ClientReady{Event: "message", Data: data}); err != nil {
		p.logger.Errorf("Fehler beim Senden von CLIENT_READY: %v", err)
	}
}

// EnableReconnect makes the pad reconnect when its connection breaks,
// trying hosts in turn (as a load balancer would pick another instance).
// Unacknowledged changes are kept and sent again after the reconnect,
// unless the server reports them as stored.
func (p *Pad) EnableReconnect(hosts []string) {
	p.poolLock.Lock()
	p.reconnectHosts = hosts
	p.poolLock.Unlock()
}

func (p *Pad) reconnect() bool {
	p.poolLock.Lock()
	hosts := p.reconnectHosts
	p.reconnecting = true
	p.poolLock.Unlock()
	p.emit("reconnecting", nil)
	deadline := time.Now().Add(60 * time.Second)
	for attempt := 0; time.Now().Before(deadline); attempt++ {
		select {
		case <-p.closeChan:
			return false
		default:
		}
		host := hosts[attempt%len(hosts)]
		state := parsePadURL(host, p.logger)
		state.PadId = p.padId
		conn, err := dialPad(state, p.logger)
		if err != nil {
			time.Sleep(time.Duration(min(attempt+1, 5)) * 200 * time.Millisecond)
			continue
		}
		p.connWrite.Lock()
		p.conn = conn
		p.host = state.Host
		p.connWrite.Unlock()
		go p.readLoop(conn)
		p.sendClientReady(conn, true)
		return true
	}
	return false
}

func (p *Pad) readLoop(connection *websocket.Conn) {
	logger := p.logger
	pad := p
	// Recover to avoid crashing the whole process on unexpected panics in the reader loop
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("panic in recv goroutine: %v", r)
			_ = connection.Close()
		}
	}()

	var (
		newline = []byte{'\n'}
		space   = []byte{' '}
	)
	for {
		select {
		case <-pad.closeChan:
			return
		default:
		}
		_, message, err := connection.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				logger.Errorf("error: %v", err)
			}
			pad.poolLock.RLock()
			canReconnect := len(pad.reconnectHosts) > 0
			pad.poolLock.RUnlock()
			select {
			case <-pad.closeChan:
				return
			default:
			}
			if canReconnect && pad.reconnect() {
				return
			}
			pad.emit("disconnect", err)
			pad.Close()
			return
		}
		message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
		logger.Debugf("Received: %s", message)

		var obj map[string]interface{}
		if err := json.Unmarshal(message, &obj); err == nil {
			if event, _ := obj["event"].(string); event == "message" {
				if data, ok := obj["data"].(map[string]interface{}); ok {
					pad.emit("message", data)
				}
			}
			continue
		}
		var arr []interface{}
		if err := json.Unmarshal(message, &arr); err != nil || len(arr) != 2 {
			continue
		}
		if msgType, _ := arr[0].(string); msgType != "message" {
			continue
		}
		msgMap, ok := arr[1].(map[string]interface{})
		if !ok {
			logger.Errorf("Fehler beim Casten der Nachricht zu map[string]interface{} (Array-Format)")
			continue
		}
		typeStr, _ := msgMap["type"].(string)
		switch typeStr {
		case "CLIENT_VARS":
			pad.handleClientVars(msgMap)
		case "COLLABROOM":
			data, ok := msgMap["data"].(map[string]interface{})
			if !ok {
				continue
			}
			switch data["type"] {
			case "NEW_CHANGES":
				pad.handleNewChanges(data)
			case "ACCEPT_COMMIT":
				pad.handleAcceptCommit(data)
			case "CLIENT_RECONNECT":
				pad.handleClientReconnect(data)
			}
		}
	}
}

func (p *Pad) handleClientVars(msgMap map[string]interface{}) {
	data, ok := msgMap["data"].(map[string]interface{})
	if !ok {
		p.logger.Errorf("CLIENT_VARS: Data fehlt oder hat falschen Typ (Array-Format)")
		return
	}
	collabVars, ok := data["collab_client_vars"].(map[string]interface{})
	if !ok {
		p.logger.Errorf("CLIENT_VARS: collab_client_vars fehlt oder hat falschen Typ (Array-Format)")
		return
	}
	initText, _ := collabVars["initialAttributedText"].(map[string]interface{})
	atext := apool.AText{
		Text:    initText["text"].(string),
		Attribs: initText["attribs"].(string),
	}
	p.emit("numConnectedUsers", collabVars["numConnectedUsers"])
	apoolMap, _ := collabVars["apool"].(map[string]interface{})
	pool := parseWirePool(apoolMap)
	// protect setting shared fields
	p.poolLock.Lock()
	p.apool = &pool
	if rev, ok := collabVars["rev"].(float64); ok {
		p.baseRev = int(rev)
	}
	p.atext = &atext
	if userId, ok := data["userId"].(string); ok {
		p.authorId = userId
	}
	p.poolLock.Unlock()
	p.emit("connected", nil)
}

func parseWirePool(apoolMap map[string]interface{}) apool.APool {
	pool := apool.NewAPool()
	if numToAttrib, ok := apoolMap["numToAttrib"].(map[string]interface{}); ok {
		for k, v := range numToAttrib {
			idx, err := strconv.Atoi(k)
			if err != nil {
				continue
			}
			if arr, ok := v.([]interface{}); ok && len(arr) == 2 {
				key, _ := arr[0].(string)
				value, _ := arr[1].(string)
				pool.NumToAttrib[idx] = apool.Attribute{Key: key, Value: value}
			}
		}
	}
	if nextNum, ok := apoolMap["nextNum"].(float64); ok {
		pool.NextNum = int(nextNum)
	}
	return pool
}

func (p *Pad) handleNewChanges(data map[string]interface{}) {
	p.poolLock.RLock()
	ready := p.apool != nil && p.atext != nil
	baseRev := p.baseRev
	p.poolLock.RUnlock()
	if !ready {
		p.logger.Errorf("received NEW_CHANGES but pad.apool or pad.atext is nil - skipping")
		return
	}
	newRev, ok := data["newRev"].(float64)
	if !ok || int(newRev) <= baseRev {
		return
	}
	if int(newRev)-1 != baseRev {
		gap := int(newRev) - 1 - baseRev
		p.logger.Warnf("missed %d revisions (server: %d, local: %d) - requesting resync", gap, int(newRev), baseRev)
		p.emit("outOfSync", map[string]interface{}{"serverRev": int(newRev), "localRev": baseRev, "gap": gap})
		return
	}
	p.applyServerChange(data, int(newRev))
}

// applyServerChange applies a revision of another author. The local text
// is the server text plus the in-flight and the outgoing changeset: those
// are rebased over the server's change, and the server's change, rebased
// over them, is applied locally.
func (p *Pad) applyServerChange(data map[string]interface{}, newRev int) {
	apoolMap, _ := data["apool"].(map[string]interface{})
	wireApool := parseWirePool(apoolMap)
	changesetStr, _ := data["changeset"].(string)

	p.poolLock.Lock()
	localPool, localAText := p.apool, p.atext
	if localPool == nil || localAText == nil {
		p.poolLock.Unlock()
		return
	}
	serverChangeset := changeset.MoveOpsToNewPool(changesetStr, &wireApool, localPool)
	server := &PadChangeset{changeset: serverChangeset}
	if p.inFlight != nil {
		transformX(p.inFlight, server, localPool)
	}
	if p.outgoing != nil {
		transformX(p.outgoing, server, localPool)
	}
	if unpacked, err := changeset.Unpack(server.changeset); err != nil || utf8.RuneCountInString(localAText.Text) != unpacked.OldLen {
		p.poolLock.Unlock()
		p.logger.Errorf("server changeset does not fit the local text - resync needed")
		p.emit("outOfSync", map[string]interface{}{"serverRev": newRev, "localRev": p.baseRev})
		return
	}
	atext, err := changeset.ApplyToAText(server.changeset, *localAText, *localPool)
	if err != nil {
		p.poolLock.Unlock()
		p.logger.Errorf("Fehler beim Anwenden des Changesets: %v", err)
		return
	}
	p.baseRev = newRev
	if p.inFlight != nil {
		p.inFlight.baseRev = newRev
	}
	if p.outgoing != nil {
		p.outgoing.baseRev = newRev
	}
	p.atext = atext
	p.poolLock.Unlock()
	p.emit("newContents", atext)
}

func (p *Pad) handleAcceptCommit(data map[string]interface{}) {
	newRev, ok := data["newRev"].(float64)
	if !ok {
		return
	}
	p.poolLock.Lock()
	if int(newRev) <= p.baseRev {
		p.poolLock.Unlock()
		return
	}
	if int(newRev)-1 != p.baseRev {
		p.poolLock.Unlock()
		p.logger.Errorf("wrong incoming revision :%v/%v", int(newRev), p.baseRev)
		return
	}
	p.acceptInFlight(int(newRev))
	p.poolLock.Unlock()
	p.emit("acceptCommit", int(newRev))
}

// acceptInFlight records the in-flight change as stored as newRev and sends
// the next one. Callers hold poolLock.
func (p *Pad) acceptInFlight(newRev int) {
	p.baseRev = newRev
	p.inFlight = nil
	if p.outgoing != nil {
		p.outgoing.baseRev = newRev
	}
	p.sendMessage(nil)
}

// handleClientReconnect follows the browser (collab_client.js): a missed
// revision of this author is the in-flight change, stored before the old
// connection broke, so it counts as accepted. Once caught up, a change the
// server never stored is sent again.
func (p *Pad) handleClientReconnect(data map[string]interface{}) {
	noChanges, _ := data["noChanges"].(bool)
	newRev, _ := data["newRev"].(float64)
	headRev, _ := data["headRev"].(float64)
	if !noChanges {
		p.poolLock.RLock()
		baseRev := p.baseRev
		mine := data["author"] == p.authorId && p.inFlight != nil
		p.poolLock.RUnlock()
		if int(newRev) != baseRev+1 {
			p.logger.Warnf("bad message revision on CLIENT_RECONNECT: %d not %d", int(newRev), baseRev+1)
			return
		}
		if mine {
			p.poolLock.Lock()
			p.baseRev = int(newRev)
			p.inFlight = nil
			if p.outgoing != nil {
				p.outgoing.baseRev = int(newRev)
			}
			p.poolLock.Unlock()
			p.emit("acceptCommit", int(newRev))
		} else {
			p.applyServerChange(data, int(newRev))
		}
		if int(newRev) != int(headRev) {
			return
		}
	}
	// Caught up: send whatever was not stored, as one change on the
	// current revision.
	p.poolLock.Lock()
	p.reconnecting = false
	if p.inFlight != nil {
		pending := p.inFlight
		p.inFlight = nil
		if p.outgoing != nil {
			if composed, err := changeset.Compose(pending.changeset, p.outgoing.changeset, p.apool); err == nil {
				pending.changeset = *composed
			}
		}
		pending.baseRev = p.baseRev
		p.outgoing = pending
	}
	p.sendMessage(nil)
	p.poolLock.Unlock()
	p.emit("reconnected", nil)
}

// transformX rebases client over server and server over client, both from
// the original pair.
func transformX(client, server *PadChangeset, pool *apool.APool) {
	origClient, origServer := client.changeset, server.changeset
	if cs, err := changeset.Follow(origServer, origClient, false, pool); err == nil && cs != nil {
		client.changeset = *cs
	}
	if cs, err := changeset.Follow(origClient, origServer, true, pool); err == nil && cs != nil {
		server.changeset = *cs
	}
}

type PadChangeset struct {
	changeset string
	baseRev   int
}

func (p *Pad) sendMessage(optMsg *PadChangeset) {
	if optMsg != nil {
		if p.outgoing != nil {
			if optMsg.baseRev != p.outgoing.baseRev {
				p.logger.Warnf("Dropping outgoing changeset due to baseRev mismatch")
				return
			}
			tempStr, err := changeset.Compose(p.outgoing.changeset, optMsg.changeset, p.apool)
			if err != nil {
				p.logger.Errorf("Error composing outgoing changesets: %v", err)
				return
			}
			p.outgoing.changeset = *tempStr
		} else {
			p.outgoing = optMsg
		}
	}
	if p.reconnecting {
		return
	}
	if p.inFlight == nil && p.outgoing != nil {
		p.inFlight = p.outgoing
		p.outgoing = nil
		apoolCreated := apool.NewAPool()
		// The wire pool numbers attributes from 0, so the changeset has to be
		// renumbered to match it.
		wireChangeset := changeset.MoveOpsToNewPool(p.inFlight.changeset, p.apool, &apoolCreated)
		wirePool := apoolCreated.ToJsonable()
		p.logger.Debugf("Sending changeset: %s", p.inFlight.changeset)
		msg := ws.UserChange{
			Event: "message",
			Data: ws.UserChangeData{
				Type:      "COLLABROOM",
				Component: "pad",
				Data: ws.UserChangeDataData{
					Type:      "USER_CHANGES",
					BaseRev:   p.inFlight.baseRev,
					Changeset: wireChangeset,
					Apool: ws.UserChangeDataDataApool{
						NumToAttrib: wirePool.NumToAttribRaw,
						NextNum:     wirePool.NextNum,
					},
				},
			},
		}
		p.connWrite.Lock()
		defer p.connWrite.Unlock()
		_ = p.conn.WriteJSON(msg)
	}
}

func (p *Pad) OnConnected(callback func(padState *Pad)) {
	p.On("connected", func(data interface{}) {
		callback(p)
	})
}

func (p *Pad) OnNumConnectedUsers(callback func(count int)) {
	p.On("numConnectedUsers", func(data interface{}) {
		if count, ok := data.(float64); ok {
			callback(int(count))
		}
	})
}

func (p *Pad) OnDisconnect(callback func(err interface{})) {
	p.On("disconnect", func(data interface{}) {
		callback(data)
	})
}

func (p *Pad) OnMessage(callback func(msg map[string]interface{})) {
	p.On("message", func(data interface{}) {
		if msg, ok := data.(map[string]interface{}); ok {
			callback(msg)
		}
	})
}

func (p *Pad) OnAcceptCommit(callback func(rev int)) {
	p.On("acceptCommit", func(data interface{}) {
		if rev, ok := data.(int); ok {
			callback(rev)
		}
	})
}

func (p *Pad) OnNewContents(callback func(atext apool.AText)) {
	p.On("newContents", func(data interface{}) {
		if atext, ok := data.(*apool.AText); ok {
			if atext != nil {
				callback(*atext)
			}
		} else {
			p.logger.Warnf("OnNewContents: invalid data type received: %T", data)
		}
	})
}

func RunFromCLI(logger *zap.SugaredLogger, args []string) {
	host, appendStr, err := parseCLIArgs(args)
	if err != nil {
		return
	}

	if host == "" {
		logger.Warnf("No host specified..")
		return
	}

	if appendStr != "" {
		pad := connect(host, logger)
		pad.OnConnected(func(_ *Pad) {
			logger.Infof("CLI Connected, appending...")
			pad.Append(appendStr)
			logger.Infof("Appended %q to %s", appendStr, host)
			if os.Getenv("GO_TEST_MODE") == "true" {
				pad.emit("append_done", nil)
			} else {
				os.Exit(0)
			}
		})
		if os.Getenv("GO_TEST_MODE") == "true" {
			done := make(chan struct{})
			pad.On("append_done", func(_ interface{}) {
				close(done)
			})
			select {
			case <-done:
				pad.Close()
				return
			case <-time.After(10 * time.Second):
				logger.Warnf("Append timeout")
				pad.Close()
				return
			}
		} else {
			select {}
		}
	} else {
		pad := connect(host, logger)
		pad.OnConnected(func(padState *Pad) {
			logger.Infof("Connected to %s with padId %s", padState.host, padState.padId)
			logger.Debugf("Pad Contents: \n%s", padState.atext.Text)
		})
		pad.OnNewContents(func(atext apool.AText) {
			logger.Debugf("Pad Contents: \n%s", atext.Text)
		})

		done := make(chan struct{})
		pad.On("disconnect", func(_ interface{}) {
			close(done)
		})
		<-done
	}

	logger.Infof("Stopping CLI")
}

func parseCLIArgs(args []string) (string, string, error) {
	fs := flag.NewFlagSet("cli", flag.ContinueOnError)
	host := fs.String("host", "", "The host of the pad (e.g. http://127.0.0.1:9001/p/test)")
	appendStr := fs.String("append", "", "Append contents to pad")
	fs.StringVar(appendStr, "a", "", "Append contents to pad (shorthand)")

	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		*host = args[0]
		args = args[1:]
	}

	err := fs.Parse(args)
	return *host, *appendStr, err
}
