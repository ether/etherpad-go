package db

type PadDBSearch struct {
	Padname        string
	LastEdited     int64
	RevisionNumber int
}

type PadDBSearchResult struct {
	TotalPads int
	Pads      []PadDBSearch
}
