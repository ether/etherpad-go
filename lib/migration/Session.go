package migration

type Session struct {
	SessionId  string `json:"-"`
	GroupId    string `json:"groupID"`
	AuthorId   string `json:"authorID"`
	ValidUntil int64  `json:"validUntil"`
}
