package admin

type PadDBSearch struct {
	LastEdited     int64  `json:"lastEdited"`
	PadName        string `json:"padName"`
	UserCount      int    `json:"userCount"`
	RevisionNumber int    `json:"revisionNumber"`
}

type PadDefinition struct {
	Total   int           `json:"total"`
	Results []PadDBSearch `json:"results"`
}
