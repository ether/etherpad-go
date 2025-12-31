package db

import "strconv"

type AText struct {
	Text    string `json:"text"`
	Attribs string `json:"attribs"`
}

type PadPool struct {
	NumToAttrib map[string][]string `json:"numToAttrib"`
	NextNum     int                 `json:"nextNum"`
}

func (p *PadPool) ToIntPool() map[int][]string {
	intPool := make(map[int][]string)
	for k, v := range p.NumToAttrib {
		convertedInteger, err := strconv.Atoi(k)
		if err != nil {
			continue
		}
		intPool[convertedInteger] = v
	}
	return intPool
}

type PadDB struct {
	RevNum         int                       `json:"head"`
	SavedRevisions map[int]PadRevision       `json:"savedRevisions"`
	Revisions      map[int]PadSingleRevision `json:"revisions"`
	ReadOnlyId     string                    `json:"readOnlyId"`
	Pool           PadPool                   `json:"pool"`
	ChatHead       int                       `json:"chatHead"`
	PublicStatus   bool                      `json:"publicStatus"`
	AText          AText                     `json:"atext"`
}

type PadRevision struct {
	Content   string
	PadDBMeta PadRevDBMeta
}

type PadSingleRevision struct {
	PadId     string
	RevNum    int
	Changeset string
	AText     AText
	AuthorId  *string
	Timestamp int64
	Pool      *RevPool
}

type PadSavedDBMeta struct {
	Author    *string
	Timestamp int64
	Pool      *RevPool
	AText     *AText
}

type PadRevDBMeta struct {
	Author    *string
	Timestamp int64
	Pool      *RevPool
	AText     *AText
}
