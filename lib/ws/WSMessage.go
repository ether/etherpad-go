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

type AccessStatusMessage struct {
	AccessStatus string `json:"accessStatus"`
}

type UserInfoUpdateWrapper struct {
	Event string         `json:"event"`
	Data  UserInfoUpdate `json:"data"`
}

type UserInfoUpdate struct {
	Type string `json:"type"`
	Data struct {
		UserInfo struct {
			ColorId *string `json:"colorId"`
			IP      *string `json:"ip"`
			Name    *string `json:"name"`
			UserId  *string `json:"userId"`
		} `json:"userInfo"`
		Type string `json:"type"`
	} `json:"data"`
}

type PadDelete struct {
	Type string `json:"type"`
	Data struct {
		PadID string `json:"padId"`
	}
}

type PadDeleteMessage struct {
	Disconnect string `json:"disconnect"`
}
