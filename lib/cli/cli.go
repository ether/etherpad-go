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
}

func NewPad(host, padId string, conn *websocket.Conn) *Pad {
	return &Pad{
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
		fmt.Println("Pad ist nicht initialisiert (atext oder apool ist nil)")
		return
	}

	if len(text) == 0 {
		fmt.Println("Kein Text zum Anhängen – Changeset wird nicht erzeugt.")
		p.poolLock.Unlock()
		return
	}

	if text == "\n" && strings.HasSuffix(p.atext.Text, "\n") {
		fmt.Println("Pad endet bereits mit Zeilenumbruch – Changeset wird nicht erzeugt.")
		p.poolLock.Unlock()
		return
	}

	if text[len(text)-1] != '\n' {
		text += "\n"
	}

	start := utf8.RuneCountInString(p.atext.Text)
	emptyAttribs := ""
	newChangeset, err := changeset.MakeSplice(p.atext.Text, start, 0, text, &emptyAttribs, p.apool)
	if err != nil {
		p.poolLock.Unlock()
		fmt.Printf("Error creating changeset: %v\n", err)
		return
	}

	// Unpack and repack to ensure canonical form
	unpacked, err := changeset.Unpack(newChangeset)
	if err != nil {
		p.poolLock.Unlock()
		fmt.Printf("Error unpacking changeset: %v\n", err)
		return
	}
	newChangeset = changeset.Pack(unpacked.OldLen, unpacked.NewLen, unpacked.Ops, unpacked.CharBank)

	// Validate generated changeset header: oldLen should equal current local text length
	if unpacked.OldLen != start {
		p.poolLock.Unlock()
		fmt.Printf("Generated changeset oldLen mismatch: expected %d got %d; not sending\n", start, unpacked.OldLen)
		// emit an error event so callers/tests can react
		p.emit("append_error", map[string]interface{}{"error": "oldLen_mismatch", "expected": start, "got": unpacked.OldLen})
		return
	}

	newAText, err := changeset.ApplyToAText(newChangeset, *p.atext, *p.apool)
	if err != nil {
		p.poolLock.Unlock()
		fmt.Printf("Error applying changeset: %v\n", err)
		return
	}

	p.atext = newAText
	baseRev := p.baseRev
	p.poolLock.Unlock()

	// Queue the changeset for sending
	pc := &PadChangeset{changeset: newChangeset, baseRev: baseRev}
	p.sendMessage(pc)
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

	padState := PadState{}

	if host == "" {
		padState.Host = "http://127.0.0.1:9001"
		padState.Path = "/p/test"
		padState.PadId = utils.RandomString(10)
	} else {
		parsedUrl, err := url.Parse(host)
		if err != nil {
			fmt.Println("Invalid host URL:", err)
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
	}

	httpClient := &http.Client{}
	fullUrl := fmt.Sprintf("%s%s/p/%s", padState.Host, padState.Path, padState.PadId)
	fmt.Printf("Getting Pad at %s\n", fullUrl)
	resp, err := httpClient.Get(fullUrl)
	if err != nil {
		fmt.Printf("Failed to connect to pad at %s: %v\n", fullUrl, err)
		os.Exit(1)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		fmt.Printf("Failed to connect to pad at %s, status: %s, body: %s\n", fullUrl, resp.Status, string(body))
		os.Exit(1)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	wsUrl := fmt.Sprintf("%s/%ssocket.io", strings.Replace(padState.Host, "http", "ws", 1), padState.Path)
	fmt.Printf("Connecting to WebSocket at %s\n", wsUrl)
	connection, resp, err := websocket.DefaultDialer.Dial(wsUrl, nil)
	if err != nil {
		fmt.Printf("WebSocket connection failed: %v\n", err)
		if resp != nil {
			fmt.Printf("Response Status: %s\n", resp.Status)
		}
		os.Exit(1)
	}

	var authorToken = "t." + utils.RandomString(20)

	pad := NewPad(padState.Host, padState.PadId, connection)

	go func() {
		// Recover to avoid crashing the whole process on unexpected panics in the reader loop
		defer func() {
			if r := recover(); r != nil {
				logger.Errorf("panic in recv goroutine: %v", r)
				pad.emit("disconnect", r)
				_ = connection.Close()
			}
			pad.Close()
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
				_, message, err := connection.ReadMessage()
				if err != nil {
					if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
						logger.Errorf("error: %v", err)
					}
					pad.emit("disconnect", err)
					return
				}
				message = bytes.TrimSpace(bytes.Replace(message, newline, space, -1))
				logger.Debugf("Received: %s", message)

				var arr []interface{}
				isArrayFormat := false
				if err := json.Unmarshal(message, &arr); err == nil && len(arr) == 2 {
					isArrayFormat = true
				}

				if isArrayFormat {
					msgType, _ := arr[0].(string)
					if msgType != "message" {
						continue

					}
					msgObj := arr[1]
					msgMap, ok := msgObj.(map[string]interface{})
					if !ok {
						logger.Errorf("Fehler beim Casten der Nachricht zu map[string]interface{} (Array-Format)")
						continue
					}
					typeStr, _ := msgMap["type"].(string)
					switch typeStr {
					case "CLIENT_VARS":
						data, ok := msgMap["data"].(map[string]interface{})
						if !ok {
							logger.Errorf("CLIENT_VARS: Data fehlt oder hat falschen Typ (Array-Format)")
							continue
						}
						collabVars, ok := data["collab_client_vars"].(map[string]interface{})
						if !ok {
							logger.Errorf("CLIENT_VARS: collab_client_vars fehlt oder hat falschen Typ (Array-Format)")
							continue
						}
						initText, _ := collabVars["initialAttributedText"].(map[string]interface{})
						atext := apool.AText{
							Text:    initText["text"].(string),
							Attribs: initText["attribs"].(string),
						}
						pad.emit("numConnectedUsers", collabVars["numConnectedUsers"])
						apoolMap, _ := collabVars["apool"].(map[string]interface{})
						pool := apool.NewAPool()
						if numToAttrib, ok := apoolMap["numToAttrib"].(map[string]interface{}); ok {
							for k, v := range numToAttrib {
								idx, err := strconv.Atoi(k)
								if err != nil {
									continue
								}
								if arr, ok := v.([]interface{}); ok && len(arr) == 2 {
									attr := apool.Attribute{
										Key:   arr[0].(string),
										Value: arr[1].(string),
									}
									pool.NumToAttrib[idx] = attr
								}
							}
						}
						if nextNum, ok := apoolMap["nextNum"].(float64); ok {
							pool.NextNum = int(nextNum)
						}
						// protect setting shared fields
						pad.poolLock.Lock()
						pad.apool = &pool
						if rev, ok := collabVars["rev"].(float64); ok {
							pad.baseRev = int(rev)
						}
						pad.atext = &atext
						pad.poolLock.Unlock()
						pad.emit("connected", nil)
					case "COLLABROOM":
						data, ok := msgMap["data"].(map[string]interface{})
						if !ok {
							continue
						}
						if data["type"] == "NEW_CHANGES" {
							// Ensure we have initial state
							pad.poolLock.RLock()
							havePool := pad.apool != nil
							haveAText := pad.atext != nil
							pad.poolLock.RUnlock()
							if !havePool || !haveAText {
								logger.Errorf("received NEW_CHANGES but pad.apool or pad.atext is nil - skipping")
								continue
							}
							if newRev, ok := data["newRev"].(float64); ok && int(newRev) <= pad.baseRev {
								continue
							}
							if newRev, ok := data["newRev"].(float64); ok {
								if int(newRev)-1 != pad.baseRev {
									logger.Errorf("wrong incoming revision :%v/%v", int(newRev), pad.baseRev)
									continue
								}
							}
							wireApool := apool.NewAPool()
							if apoolMap, ok := data["apool"].(map[string]interface{}); ok {
								if numToAttrib, ok := apoolMap["numToAttrib"].(map[string]interface{}); ok {
									for k, v := range numToAttrib {
										idx, err := strconv.Atoi(k)
										if err != nil {
											continue
										}
										if arr, ok := v.([]interface{}); ok && len(arr) == 2 {
											attr := apool.Attribute{
												Key:   arr[0].(string),
												Value: arr[1].(string),
											}
											wireApool.NumToAttrib[idx] = attr
										}
									}
								}
								if nextNum, ok := apoolMap["nextNum"].(float64); ok {
									wireApool.NextNum = int(nextNum)
								}
							}
							changesetStr, _ := data["changeset"].(string)
							// Re-read pad.apool and pad.atext under read lock to ensure stability
							pad.poolLock.RLock()
							localPool := pad.apool
							localAText := pad.atext
							pad.poolLock.RUnlock()
							if localPool == nil || localAText == nil {
								logger.Errorf("pad.apool or pad.atext became nil while processing NEW_CHANGES - skipping")
								continue
							}
							serverChangeset := changeset.MoveOpsToNewPool(changesetStr, &wireApool, localPool)
							server := &PadChangeset{changeset: serverChangeset}
							// Validate server changeset header before attempting to apply it
							if unpacked, err := changeset.Unpack(server.changeset); err != nil {
								logger.Errorf("cannot unpack server changeset: %v - skipping", err)
								continue
							} else if utf8.RuneCountInString(localAText.Text) != unpacked.OldLen {
								logger.Errorf("server changeset oldLen %d does not match local text length %d - skipping", unpacked.OldLen, utf8.RuneCountInString(localAText.Text))
								continue
							}
							if pad.inFlight != nil {
								transformX(pad.inFlight, server, localPool)
							}
							if pad.outgoing != nil {
								transformX(pad.outgoing, server, localPool)
								if newRev, ok := data["newRev"].(float64); ok {
									pad.outgoing.baseRev = int(newRev)
								}
							}
							atext, err := changeset.ApplyToAText(server.changeset, *localAText, *localPool)
							if err != nil {
								logger.Errorf("Fehler beim Anwenden des Changesets: %v", err)
								continue
							}
							// write back updated atext under lock
							pad.poolLock.Lock()
							pad.atext = atext
							pad.poolLock.Unlock()
							if newRev, ok := data["newRev"].(float64); ok {
								pad.baseRev = int(newRev)
							}
							pad.emit("newContents", atext)
						}
						if data["type"] == "ACCEPT_COMMIT" {
							if newRev, ok := data["newRev"].(float64); ok && int(newRev) <= pad.baseRev {
								continue
							}
							if newRev, ok := data["newRev"].(float64); ok {
								if int(newRev)-1 != pad.baseRev {
									logger.Errorf("wrong incoming revision :%v/%v", int(newRev), pad.baseRev)
									continue
								}
								pad.baseRev = int(newRev)
								pad.inFlight = nil
								if pad.outgoing != nil {
									pad.outgoing.baseRev = int(newRev)
								}
								pad.sendMessage(nil)
							}
						}
					}
				}
				var obj map[string]interface{}
				if err := json.Unmarshal(message, &obj); err == nil {
					event, _ := obj["event"].(string)
					if event == "message" {
						data, ok := obj["data"].(map[string]interface{})
						if ok {
							typeStr, _ := data["type"].(string)
							if typeStr == "CLIENT_READY" {
								pad.emit("connected", nil)
							}
							pad.emit("message", data)
						}
					}
				}
			}
		}
	}()

	if err := connection.WriteJSON(ws.ClientReady{
		Event: "message",
		Data: ws.ClientReadyData{
			Component: "pad",
			Type:      "CLIENT_READY",
			PadID:     padState.PadId,
			Token:     authorToken,
			UserInfo: ws.ClientReadyUserInfo{
				ColorId: nil,
				Name:    nil,
			},
		},
	}); err != nil {
		logger.Errorf("Fehler beim Senden von CLIENT_READY: %v", err)
	}

	return pad
}

func transformX(client, server *PadChangeset, pool *apool.APool) {
	if cs, err := changeset.Follow(server.changeset, client.changeset, false, pool); err == nil && cs != nil {
		client.changeset = *cs
	}
	if cs, err := changeset.Follow(client.changeset, server.changeset, true, pool); err == nil && cs != nil {
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
				fmt.Println("Dropping outgoing changeset due to baseRev mismatch")
				return
			}
			tempStr, err := changeset.Compose(p.outgoing.changeset, optMsg.changeset, p.apool)
			if err != nil {
				fmt.Printf("Error composing outgoing changesets: %v\n", err)
				return
			}
			p.outgoing.changeset = *tempStr
		} else {
			p.outgoing = optMsg
		}
	}
	if p.inFlight == nil && p.outgoing != nil {
		p.inFlight = p.outgoing
		p.outgoing = nil
		apoolCreated := apool.NewAPool()
		changeset.MoveOpsToNewPool(p.inFlight.changeset, p.apool, &apoolCreated)
		wirePool := apoolCreated.ToJsonable()
		fmt.Println("Sending changeset:", p.inFlight.changeset)
		msg := ws.UserChange{
			Event: "message",
			Data: ws.UserChangeData{
				Type:      "COLLABROOM",
				Component: "pad",
				Data: ws.UserChangeDataData{
					Type:      "USER_CHANGES",
					BaseRev:   p.inFlight.baseRev,
					Changeset: p.inFlight.changeset,
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

func (p *Pad) OnNewContents(callback func(atext apool.AText)) {
	p.On("newContents", func(data interface{}) {
		if atext, ok := data.(*apool.AText); ok {
			if atext != nil {
				callback(*atext)
			}
		} else {
			println("OnNewContents: invalid data type received")
		}
	})
}

func RunFromCLI(logger *zap.SugaredLogger, args []string) {
	host, appendStr, err := parseCLIArgs(args)
	if err != nil {
		return
	}

	if host == "" {
		fmt.Println("No host specified..")
		return
	}

	if appendStr != "" {
		pad := connect(host, logger)
		pad.OnConnected(func(_ *Pad) {
			fmt.Println("CLI Connected, appending...")
			pad.Append(appendStr)
			fmt.Printf("Appended %q to %s\n", appendStr, host)
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
				fmt.Println("Append timeout")
				pad.Close()
				return
			}
		} else {
			select {}
		}
	} else {
		pad := connect(host, logger)
		pad.OnConnected(func(padState *Pad) {
			fmt.Printf("Connected to %s with padId %s\n", padState.host, padState.padId)
			fmt.Print("\u001b[2J\u001b[0;0H")
			if padState.atext != nil {
				fmt.Println("Pad Contents", "\n"+padState.atext.Text)
			}
		})
		pad.OnNewContents(func(atext apool.AText) {
			fmt.Print("\u001b[2J\u001b[0;0H")
			fmt.Println("Pad Contents", "\n"+atext.Text)
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
