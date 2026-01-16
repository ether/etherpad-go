package cli

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/utils"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

// Dummy Pad struct und Methoden für Demo-Zwecke
// In echter Implementierung: WebSocket/HTTP-Client für Etherpad

type Pad struct {
	host  string
	padId string
	atext string
}

type PadState struct {
	Host  string
	Path  string
	PadId string
}

func connect(host string) *Pad {

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
	resp, err := httpClient.Get(fmt.Sprintf("%s%s/p/%s", padState.Host, padState.Path, padState.PadId))
	if err != nil || resp.StatusCode != http.StatusOK {
		fmt.Printf("Failed to connect to pad at %s%s/p/%s\n", padState.Host, padState.Path, padState.PadId)
		os.Exit(1)
	}

	websocket.DefaultDialer.Dial()
}

func (p *Pad) OnConnected(callback func(padState *Pad)) {
	// Simuliere initialen Verbindungsaufbau
	callback(p)
}

func (p *Pad) OnNewContents(callback func(atext string)) {
	// Simuliere neue Inhalte (hier: alle 3 Sekunden Dummy-Update)
	for i := 0; i < 3; i++ {
		time.Sleep(3 * time.Second)
		p.atext = fmt.Sprintf("Demo Pad Inhalt Update %d", i+1)
		callback(p.atext)
	}
}

func (p *Pad) Append(s string) {
	p.atext += s
}

func StartCLI(logger *zap.SugaredLogger) {
	args := os.Args
	if len(args) < 3 {
		fmt.Println("No host specified..")
		fmt.Println("Stream Pad to CLI: etherpad http://127.0.0.1:9001/p/test")
		fmt.Println("Append contents to pad: etherpad http://127.0.0.1:9001/p/test -a 'hello world'")
		os.Exit(1)
	}

	host := args[2]
	action := ""
	if len(args) > 3 {
		action = args[3]
	}
	str := ""
	if len(args) > 4 {
		str = args[4]
	}

	if host != "" {
		if action == "" {
			pad := connect(host)
			pad.OnConnected(func(padState *Pad) {
				fmt.Printf("Connected to %s with padId %s\n", padState.host, padState.padId)
				fmt.Print("\u001b[2J\u001b[0;0H")
				fmt.Println("Pad Contents", "\n"+padState.atext)
			})
			pad.OnNewContents(func(atext string) {
				fmt.Print("\u001b[2J\u001b[0;0H")
				fmt.Println("Pad Contents", "\n"+atext)
			})
		}
		if action == "-a" {
			if str == "" {
				fmt.Println("No string specified with pad")
				os.Exit(1)
			}
			pad := connect(host)
			pad.OnConnected(func(_ *Pad) {
				pad.Append(str)
				fmt.Printf("Appended %q to %s\n", str, host)
				os.Exit(0)
			})
		}
	}
}
