package admin

type EventMessage struct {
	Event string `json:"event"`
	Data  string `json:"data"`
}

type PadLoadData struct {
	Offset    int    `json:"offset"`
	Limit     int    `json:"limit"`
	Pattern   string `json:"pattern"`
	SortBy    string `json:"sortBy"`
	Ascending bool   `json:"ascending"`
}
