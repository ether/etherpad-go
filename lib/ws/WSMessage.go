package ws

import (
	"github.com/ether/etherpad-go/lib/apool"
	clientVars2 "github.com/ether/etherpad-go/lib/models/clientVars"
)

type Message struct {
	Data clientVars2.ClientVars `json:"data"`
	Type string                 `json:"type"`
}

type UserDupMessage struct {
	Disconnect string `json:"disconnect"`
}

type AcceptCommitData struct {
	Type   string `json:"type"`
	NewRev int    `json:"newRev"`
}

type AcceptCommitMessage struct {
	Type string           `json:"type"`
	Data AcceptCommitData `json:"data"`
}

type NewChangesMessageData struct {
	Type        string      `json:"type"`
	NewRev      int         `json:"newRev"`
	Changeset   string      `json:"changeset"`
	APool       apool.APool `json:"apool"`
	Author      string      `json:"author"`
	CurrentTime int         `json:"currentTime"`
	TimeDelta   int         `json:"timeDelta"`
}

type NewChangesMessage struct {
	Type string                `json:"type"`
	Data NewChangesMessageData `json:"data"`
}
