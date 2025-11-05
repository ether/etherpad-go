package db

type ChatMessageDB struct {
	PadId    string
	Head     int
	Message  string
	Time     *int64
	AuthorId *string
}

type ChatMessageDBWithDisplayName struct {
	ChatMessageDB
	DisplayName *string
}
