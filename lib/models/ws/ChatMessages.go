package ws

type GetChatMessages struct {
	Event string `json:"event"`
	Data  struct {
		Type      string `json:"type"`
		Component string `json:"component"`
		Data      struct {
			Type  string `json:"type"`
			Start int    `json:"start"`
			End   int    `json:"end"`
		} `json:"data"`
	}
}

type GetChatMessagesResponse struct {
	Type string `json:"type"`
	Data struct {
		Type     string                `json:"type"`
		Messages []ChatMessageSendData `json:"messages"`
	} `json:"data"`
}
