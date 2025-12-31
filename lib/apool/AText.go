package apool

import "github.com/ether/etherpad-go/lib/models/db"

type AText struct {
	Text    string `json:"text"`
	Attribs string `json:"attribs"`
}

func FromDBAText(dbAtext db.AText) AText {
	return AText{
		Text:    dbAtext.Text,
		Attribs: dbAtext.Attribs,
	}
}

func (a *AText) ToDBAText() db.AText {
	return db.AText{
		Text:    a.Text,
		Attribs: a.Attribs,
	}
}

func CopyAText(atext1 AText, atext2 *AText) {
	atext2.Attribs = atext1.Attribs
	atext2.Text = atext1.Text
}

func ATextsEqual(atext1 AText, atext2 AText) bool {
	return atext1.Text == atext2.Text && atext1.Attribs == atext2.Attribs
}
