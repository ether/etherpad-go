package db

import (
	"encoding/json"
	"fmt"

	"github.com/ether/etherpad-go/lib/models/db"
)

type Reader interface {
	Scan(dest ...any) error
}

func ReadToAuthorDB(reader Reader) (*db.AuthorDB, error) {
	var author db.AuthorDB

	if err := reader.Scan(&author.ID, &author.ColorId, &author.Name,
		&author.Timestamp, &author.Token, &author.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &author, nil
}

func ReadToPadDB(reader Reader) (*db.PadDB, error) {
	var padDB db.PadDB
	var savedRevisions, pool []byte

	if err := reader.Scan(&padDB.ID, &padDB.Head, &savedRevisions, &padDB.ReadOnlyId, &pool,
		&padDB.ChatHead, &padDB.PublicStatus, &padDB.ATextText, &padDB.ATextAttribs,
		&padDB.CreatedAt, &padDB.UpdatedAt); err != nil {
		return nil, err
	}
	if err := json.Unmarshal(savedRevisions, &padDB.SavedRevisions); err != nil {
		return nil, fmt.Errorf("error unmarshaling saved revisions: %w", err)
	}
	if err := json.Unmarshal(pool, &padDB.Pool); err != nil {
		return nil, fmt.Errorf("error unmarshaling pool: %w", err)
	}
	return &padDB, nil
}
