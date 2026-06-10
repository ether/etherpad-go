package ws

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
}
