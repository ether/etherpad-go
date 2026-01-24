package migration

type Author struct {
	Id        string         `json:"-"`
	ColorId   int            `json:"colorId"`
	Name      string         `json:"name"`
	Timestamp int64          `json:"timestamp"`
	PadIDs    map[string]int `json:"padIDs"`
}

type Token2Author struct {
	Token    string
	AuthorId string
}

type Author2Sessions struct {
	AuthorId string
	Sessions map[string]int
}
