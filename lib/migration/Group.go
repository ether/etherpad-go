package migration

type Group struct {
	GroupId string         `json:"-"`
	Pads    map[string]int `json:"pads"`
}

type Group2Sessions struct {
	GroupId  string
	Sessions map[string]int
}
