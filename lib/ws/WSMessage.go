package ws

import "github.com/ether/etherpad-go/lib/settings"

type Message struct {
	Data settings.ClientVars `json:"data"`
	Type string              `json:"type"`
}
