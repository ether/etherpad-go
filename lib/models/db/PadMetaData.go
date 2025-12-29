package db

type PadMetaData struct {
	Id           string
	RevNum       int
	ChangeSet    string
	Atext        AText
	AtextAttribs string
	AuthorId     *string
	Timestamp    int64
}
