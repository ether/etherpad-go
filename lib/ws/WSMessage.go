package ws

import (
	clientVars2 "github.com/ether/etherpad-go/lib/models/clientVars"
)

type Message struct {
	Data clientVars2.ClientVars `json:"data"`
	Type string                 `json:"type"`
}
