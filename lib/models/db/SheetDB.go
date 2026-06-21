package db

import "time"

// SheetDB is the persisted header of a spreadsheet document (keyed by pad id).
// Snapshot is a marshaled sheet.WorkbookSnapshot.
type SheetDB struct {
	ID        string
	Head      int
	Snapshot  string
	CreatedAt time.Time
	UpdatedAt *time.Time
}

// SheetOpDB is one persisted operation in a sheet document's op-log.
type SheetOpDB struct {
	PadId     string
	Rev       int
	Op        string
	AuthorId  *string
	Timestamp int64
}
