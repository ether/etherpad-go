package ws

type ChatMessage struct {
	Event string `json:"event"`
	Data  struct {
		Type      string `json:"type"`
		Component string `json:"component"`
		Data      struct {
			Type    string          `json:"type"`
			Message ChatMessageData `json:"message"`
		}
	}
}

type ChatBroadCastMessage struct {
	Type string `json:"type"`
	Data struct {
		Type    string               `json:"type"`
		Message ChatMessageSendEvent `json:"message"`
	} `json:"data"`
}

type ChatMessageSendEvent struct {
	Text     string  `json:"text"`
	Time     *int64  `json:"time,omitempty"`
	UserId   *string `json:"userId,omitempty"`
	UserName *string `json:"userName"`
}

type ChatMessageData struct {
	Text        string  `json:"text"`
	Time        *int64  `json:"time,omitempty"`
	UserId      *string `json:"userId,omitempty"`
	AuthorId    *string `json:"authorId,omitempty"`
	DisplayName *string `json:"displayName,omitempty"`
	UserName    *string `json:"userName,omitempty"`
}

func FromObject(original ChatMessageData) ChatMessageData {
	// The userId property was renamed to authorId, and userName was renamed to displayName. Accept
	// the old names in case the db record was written by an older version of Etherpad.
	if original.UserId != nil && original.AuthorId == nil {
		original.AuthorId = original.UserId
	}

	original.UserId = nil
	if original.UserName != nil && original.DisplayName == nil {
		original.DisplayName = original.UserName
	}
	original.UserName = nil

	return original
}
