package events

// UserJoinLeaveContext is passed to userJoin and userLeave hooks.
type UserJoinLeaveContext struct {
	PadId    string
	AuthorId string
	// BroadcastChat sends a chat message to all clients in the pad room without persisting it.
	// The message map is serialized as the "message" field inside a CHAT_MESSAGE COLLABROOM event.
	BroadcastChat func(message map[string]any)
}
