package ws

type UserChange struct {
	Event string         `json:"event"`
	Data  UserChangeData `json:"data"`
}

type UserChangeData struct {
	Component string             `json:"component"`
	Data      UserChangeDataData `json:"data"`
	Type      string             `json:"type"`
}

type UserChangeDataDataApool struct {
	NumToAttrib map[int][]string `json:"numToAttrib"`
	NextNum     int              `json:"nextNum"`
}

type UserChangeDataData struct {
	Type      string                  `json:"type"`
	Apool     UserChangeDataDataApool `json:"apool"`
	BaseRev   int                     `json:"baseRev"`
	Changeset string                  `json:"changeset"`
}
