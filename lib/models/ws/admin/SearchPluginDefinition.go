package admin

type SearchPluginDefinitionQuery struct {
	Offset     int    `json:"offset"`
	Limit      int    `json:"limit"`
	SortBy     string `json:"sortBy"`
	SortDir    string `json:"sortDir"`
	SearchTerm string `json:"searchTerm"`
}

type SeachchPluginDefinition struct {
	Query   SearchPluginDefinitionQuery `json:"query"`
	Results []PluginSearchDefinition    `json:"results"`
}
