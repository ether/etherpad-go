package migration

type PadRevision struct {
	PadRevisionId string `json:"-"`
	RevNum        int    `json:"-"`
	Changeset     string `json:"changeset"`
	Meta          struct {
		Author    string `json:"author"`
		Timestamp int64  `json:"timestamp"`
		Pool      struct {
			NumToAttrib map[string][]string `json:"numToAttrib"`
			AttribToNum map[string]int      `json:"attribToNum"`
			NextNum     int                 `json:"nextNum"`
		} `json:"pool"`
		Atext struct {
			Text    string `json:"text"`
			Attribs string `json:"attribs"`
		} `json:"atext"`
	} `json:"meta"`
}
