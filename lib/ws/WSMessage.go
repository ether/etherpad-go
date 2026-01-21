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
	CurrentTime int64       `json:"currentTime"`
	TimeDelta   int64       `json:"timeDelta"`
}

type NewChangesMessage struct {
	Type string                `json:"type"`
	Data NewChangesMessageData `json:"data"`
}

type ClientReconnectData struct {
	Type        string      `json:"type"`
	HeadRev     int         `json:"headRev,omitempty"`
	NewRev      int         `json:"newRev"`
	Changeset   string      `json:"changeset,omitempty"`
	APool       apool.APool `json:"apool,omitempty"`
	Author      string      `json:"author,omitempty"`
	CurrentTime int64       `json:"currentTime,omitempty"`
	NoChanges   bool        `json:"noChanges,omitempty"`
}

type ClientReconnectMessage struct {
	Type string              `json:"type"`
	Data ClientReconnectData `json:"data"`
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

type SavedRevision struct {
	Event string            `json:"event"`
	Data  SavedRevisionData `json:"data"`
}

type SavedRevisionData struct {
	Type      string                `json:"type"`
	Component string                `json:"component"`
	Data      SavedRevisionDataData `json:"data"`
}

type SavedRevisionDataData struct {
	Type string `json:"type"`
}

type PadDeleteMessage struct {
	Disconnect string `json:"disconnect"`
}

type ChangesetResponse struct {
	Type string        `json:"type"`
	Data ChangesetInfo `json:"data"`
}

type ChangesetInfo struct {
	ForwardsChangesets  []string    `json:"forwardsChangesets"`
	BackwardsChangesets []string    `json:"backwardsChangesets"`
	ActualEndNum        int         `json:"actualEndNum"`
	TimeDeltas          []int64     `json:"timeDeltas"`
	Start               int         `json:"start"`
	Granularity         int         `json:"granularity"`
	APool               apool.APool `json:"apool"`
	RequestId           int         `json:"requestID"`
}
