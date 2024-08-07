package db

import (
	"github.com/ether/etherpad-go/lib/apool"
)

type PadDB struct {
	RevNum         int                 `json:"head"`
	SavedRevisions map[int]PadRevision `json:"savedRevisions"`
	ReadOnlyId     string              `json:"readOnlyId"`
	Pool           apool.APool         `json:"pool"`
	ChatHead       int                 `json:"chatHead"`
	PublicStatus   bool                `json:"publicStatus"`
	AText          apool.AText         `json:"atext"`
}

type PadRevision struct {
	Content   string
	PadDBMeta PadDBMeta
}

type PadSingleRevision struct {
	PadId     string
	RevNum    int
	Changeset string
	AText     apool.AText
	AuthorId  *string
	Timestamp int
}

type PadDBMeta struct {
	Author    *string
	Timestamp int
	Pool      *apool.APool
	AText     *apool.AText
}
