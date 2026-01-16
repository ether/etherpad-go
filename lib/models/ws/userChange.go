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

type UserChangeDataData struct {
	Apool struct {
		NumToAttrib map[int][]string `json:"numToAttrib"`
		NextNum     int              `json:"nextNum"`
	} `json:"apool"`
	BaseRev   int    `json:"baseRev"`
	Changeset string `json:"changeset"`
}
