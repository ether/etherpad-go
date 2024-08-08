package ws

type UserChange struct {
	Event string `json:"event"`
	Data  struct {
		Component string `json:"component"`
		Data      struct {
			Apool struct {
				NumToAttrib map[int][]string `json:"numToAttrib"`
				NextNum     int              `json:"nextNum"`
			} `json:"apool"`
			BaseRev   int    `json:"baseRev"`
			Changeset string `json:"changeset"`
		} `json:"data"`
		Type string `json:"type"`
	} `json:"data"`
}
