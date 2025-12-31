package db

import db2 "github.com/ether/etherpad-go/lib/models/db"

func CreateRandomPad() db2.PadDB {
	return db2.PadDB{
		RevNum:         0,
		Revisions:      make(map[int]db2.PadSingleRevision),
		SavedRevisions: make(map[int]db2.PadRevision),
		ReadOnlyId:     "randomReadOnlyId",
		ChatHead:       0,
		PublicStatus:   true,
		AText: struct {
			Text    string `json:"text"`
			Attribs string `json:"attribs"`
		}{Text: "df,süd,füsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdf\n", Attribs: "*0+1i|1+1"},
	}
}
