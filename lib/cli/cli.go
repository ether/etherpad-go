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
	if text[len(text)-1] != '\n' {
		text += "\n"
	}

	newChangeset, err := changeset.MakeSplice(p.atext.Text, len(p.atext.Text), 0, text, nil, nil)
	if err != nil {
		fmt.Printf("Error creating changeset: %v\n", err)
		return
	}
	newRev := p.baseRev

	p.atext, err = changeset.ApplyToAText(newChangeset, *p.atext, *p.apool)
	if err != nil {
		fmt.Printf("Error applying changeset: %v\n", err)
		return
	}
	tempPool := apool.NewAPool()
	wireApool := tempPool.ToJsonable()

	err = p.conn.WriteJSON(ws.UserChange{
		Event: "message",
		Data: ws.UserChangeData{
			Component: "pad",
			Type:      "USER_CHANGES",
			Data: ws.UserChangeDataData{
				Apool: struct {
					NumToAttrib map[int][]string `json:"numToAttrib"`
					NextNum     int              `json:"nextNum"`
				}{NumToAttrib: wireApool.NumToAttribRaw, NextNum: wireApool.NextNum},
				BaseRev:   newRev,
				Changeset: newChangeset,
			},
		},
	})

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
		defer pad.Close()
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
						pad.apool = &pool
						if rev, ok := collabVars["rev"].(float64); ok {
							pad.baseRev = int(rev)
						}
						pad.atext = &atext
						pad.emit("connected", nil)
					case "COLLABROOM":
						data, ok := msgMap["data"].(map[string]interface{})
						if !ok {
							continue
						}
						if data["type"] == "NEW_CHANGES" {
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
							serverChangeset := changeset.MoveOpsToNewPool(changesetStr, &wireApool, pad.apool)
							server := &PadChangeset{changeset: serverChangeset}
							if pad.inFlight != nil {
								transformX(pad.inFlight, server, pad.apool)
							}
							if pad.outgoing != nil {
								transformX(pad.outgoing, server, pad.apool)
								if newRev, ok := data["newRev"].(float64); ok {
									pad.outgoing.baseRev = int(newRev)
								}
							}
							atext, err := changeset.ApplyToAText(server.changeset, *pad.atext, *pad.apool)
							if err != nil {
								logger.Errorf("Fehler beim Anwenden des Changesets: %v", err)
								continue
							}
							pad.atext = atext
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
				return
			}
			if cs, err := changeset.Compose(p.outgoing.changeset, optMsg.changeset, p.apool); err == nil && cs != nil {
				p.outgoing.changeset = *cs
			}
		} else {
			p.outgoing = optMsg
		}
	}
	if p.inFlight == nil && p.outgoing != nil {
		p.inFlight = p.outgoing
		p.outgoing = nil
		msg := map[string]interface{}{
			"type":      "COLLABROOM",
			"component": "pad",
			"data": map[string]interface{}{
				"type":      "USER_CHANGES",
				"baseRev":   p.inFlight.baseRev,
				"changeset": p.inFlight.changeset,
				"apool":     p.apool.ToJsonable(),
			},
		}
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
