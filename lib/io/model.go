package io

import "encoding/json"

type AText struct {
	Text    string `json:"text"`
	Attribs string `json:"attribs"`
}

type Pool struct {
	NumToAttrib map[string][]string `json:"numToAttrib"`
	NextNum     int                 `json:"nextNum"`
}

type PoolWithAttribToNum struct {
	NumToAttrib map[string][]string    `json:"numToAttrib"`
	AttribToNum map[string]interface{} `json:"attribToNum"`
	NextNum     int                    `json:"nextNum"`
}

type PadData struct {
	AText          AText `json:"atext"`
	Pool           Pool  `json:"pool"`
	Head           int   `json:"head"`
	ChatHead       int   `json:"chatHead"`
	PublicStatus   bool  `json:"publicStatus"`
	SavedRevisions []any `json:"savedRevisions"`
}

type GlobalAuthor struct {
	ColorId   string              `json:"colorId"`
	Timestamp int64               `json:"timestamp"`
	PadIDs    map[string]struct{} `json:"padIDs"`
	Name      *string             `json:"name"`
}

type RevisionMeta struct {
	Author    *string              `json:"author"`
	Timestamp *int64               `json:"timestamp"`
	Pool      *PoolWithAttribToNum `json:"pool,omitempty"`
	AText     *AText               `json:"atext,omitempty"`
}

type Revision struct {
	Changeset string       `json:"changeset"`
	Meta      RevisionMeta `json:"meta"`
}

type ChatMessage struct {
	Text     string  `json:"text"`
	Time     *int64  `json:"time"`
	UserId   *string `json:"userId"`
	UserName *string `json:"userName"`
}

type EtherpadExport struct {
	Pad       map[string]PadData      `json:"-"`
	Authors   map[string]GlobalAuthor `json:"-"`
	Chats     map[string]ChatMessage  `json:"-"`
	Revisions map[string]Revision     `json:"-"`
}

func (e EtherpadExport) MarshalJSON() ([]byte, error) {
	combined := make(map[string]any)
	for k, v := range e.Pad {
		combined[k] = v
	}
	for k, v := range e.Authors {
		combined[k] = v
	}
	for k, v := range e.Revisions {
		combined[k] = v
	}
	for k, v := range e.Chats {
		combined[k] = v
	}
	return json.Marshal(combined)
}
