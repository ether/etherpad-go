package migration

type Pad struct {
	PadId string `json:"-"`
	AText struct {
		Text    string `json:"text"`
		Attribs string `json:"attribs"`
	} `json:"atext"`
	Pool struct {
		NumToAttrib map[string][]string `json:"numToAttrib"`
		NextNum     int                 `json:"nextNum"`
	} `json:"pool"`
	Head           int  `json:"head"`
	ChatHead       int  `json:"chatHead"`
	PublicStatus   bool `json:"publicStatus"`
	SavedRevisions []struct {
		RevNum    int    `json:"revNum"`
		Timestamp int64  `json:"timestamp"`
		SavedById string `json:"savedById"`
		Label     string `json:"label"`
		Id        string `json:"id"`
	} `json:"savedRevisions"`
}
