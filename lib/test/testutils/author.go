package testutils

import "github.com/ether/etherpad-go/lib/models/db"
import "github.com/brianvoe/gofakeit/v7"

func GenerateDBAuthor() db.AuthorDB {
	fakedName := gofakeit.Name()
	padIds := make(map[string]struct{})
	padIds["test"] = struct{}{}

	return db.AuthorDB{
		Name:      &fakedName,
		ColorId:   "#ffff",
		ID:        gofakeit.UUID(),
		Timestamp: gofakeit.Date().UnixMilli(),
	}
}
