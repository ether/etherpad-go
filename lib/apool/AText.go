package apool

type AText struct {
	Text    string `json:"text"`
	Attribs string `json:"attribs"`
}

func CopyAText(atext1 AText, atext2 AText) {
	atext2.Attribs = atext1.Attribs
	atext2.Text = atext1.Text
}
