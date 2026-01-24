package migration

type ChatMessage struct {
	PadId     string `json:"-"`
	ChatNum   int    `json:"-"`
	Text      string `json:"text"`
	AuthorId  string `json:"authorId"`
	Timestamp int64  `json:"time"`
}
