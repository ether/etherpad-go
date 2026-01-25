package author

import (
	"github.com/brianvoe/gofakeit/v7"
	"github.com/ether/etherpad-go/lib/models/db"
)

func NewRandomAuthor() *Author {
	authorName := gofakeit.Name()
	var author = Author{
		Id:        gofakeit.ID(),
		Name:      &authorName,
		Timestamp: gofakeit.Int64(),
		ColorId:   gofakeit.HexColor(),
	}
	return &author
}

func ToDBAuthor(author *Author) *db.AuthorDB {
	return &db.AuthorDB{
		ID:        author.Id,
		Name:      author.Name,
		Timestamp: author.Timestamp,
		ColorId:   author.ColorId,
		Token:     author.Token,
	}
}
