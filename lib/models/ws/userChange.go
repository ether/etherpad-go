package ws

type UserChange struct {
	Event string `json:"event"`
	Data  struct {
		Apool struct {
			NumToAttrib map[int][]string `json:"numToAttrib"`
			NextNum     int
		} `json:"apool"`
		BaseRev   int    `json:"baseRev"`
		Changeset string `json:"changeset"`
		Type      string `json:"type"`
	} `json:"data"`
}
