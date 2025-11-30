package author

import (
	"github.com/brianvoe/gofakeit/v7"
)

func NewRandomAuthor() *Author {
	authorName := gofakeit.Name()
	var author = Author{
		Id:        gofakeit.ID(),
		Name:      &authorName,
		Timestamp: gofakeit.Int64(),
		ColorId:   gofakeit.HexColor(),
		PadIDs:    make(map[string]struct{}),
	}
	return &author
}
