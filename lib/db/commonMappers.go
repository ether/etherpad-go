package db

import "github.com/ether/etherpad-go/lib/models/db"

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
