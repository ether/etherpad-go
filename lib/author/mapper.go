package author

import "github.com/ether/etherpad-go/lib/models/db"

func MapToDB(author Author) db.AuthorDB {
	return db.AuthorDB{
		ID:        author.Id,
		Token:     author.Token,
		ColorId:   author.ColorId,
		Name:      author.Name,
		Timestamp: author.Timestamp,
		// Pad ids are managed separately
	}
}

func MapFromDB(authorDB db.AuthorDB) Author {
	return Author{
		Id:        authorDB.ID,
		Token:     authorDB.Token,
		ColorId:   authorDB.ColorId,
		Name:      authorDB.Name,
		Timestamp: authorDB.Timestamp,
	}
}
