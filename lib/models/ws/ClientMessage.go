package ws

import "encoding/json"

// ClientMessage is the COLLABROOM CLIENT_MESSAGE family from the original
// protocol. The payload type selects the sub-message: "suggestUserName"
// relays a name suggestion to an unnamed user, "padoptions" distributes
// pad-wide settings.
type ClientMessage struct {
	Event string            `json:"event"`
	Data  ClientMessageData `json:"data"`
}

type ClientMessageData struct {
	Component string                `json:"component"`
	Type      string                `json:"type"`
	Data      ClientMessageDataData `json:"data"`
}

type ClientMessageDataData struct {
	Type    string               `json:"type"`
	Payload ClientMessagePayload `json:"payload"`
}

type ClientMessagePayload struct {
	Type      string         `json:"type"`
	NewName   string         `json:"newName,omitempty"`
	UnnamedId string         `json:"unnamedId,omitempty"`
	Options   map[string]any `json:"options,omitempty"`
	// Extra keeps the remaining fields, e.g. padoptions' changedBy.
	Extra map[string]any `json:"-"`
}

// UnmarshalJSON fills the known fields and keeps every field in Extra.
func (c *ClientMessagePayload) UnmarshalJSON(b []byte) error {
	type plain ClientMessagePayload
	if err := json.Unmarshal(b, (*plain)(c)); err != nil {
		return err
	}
	return json.Unmarshal(b, &c.Extra)
}
