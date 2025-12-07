package admin

type ShoutMessage struct {
	Message   ShoutMessageRequest `json:"message"`
	Timestamp int64               `json:"timestamp"`
}

type ShoutMessageRequest struct {
	Message string `json:"message"`
	Sticky  bool   `json:"sticky"`
}

type ShoutMessageResponse struct {
	Type string `json:"type"`
	Data struct {
		Type    string       `json:"type"`
		Payload ShoutMessage `json:"payload"`
	} `json:"data"`
}
