package db

import (
	"time"

	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/test/testutils/general"
)

func RandomString(length int) string {
	return general.RandomInlineString(length)
}

func CreateRandomPad() db2.PadDB {
	var randomId = "r." + RandomString(16)
	updatedAt := time.Now()
	return db2.PadDB{
		Head:           0,
		SavedRevisions: make([]db2.SavedRevision, 0),
		ReadOnlyId:     &randomId,
		ChatHead:       0,
		PublicStatus:   true,
		ATextText:      "df,süd,füsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdfsdf\n",
		ATextAttribs:   "*0+1i|1+1",
		CreatedAt:      time.Now(),
		UpdatedAt:      &updatedAt,
		ID:             "pad_" + time.Now().Format("20060102150405"),
		Pool: db2.RevPool{
			NumToAttrib: map[string][]string{
				"0": {},
				"1": {"author|randomAuthorId"},
			},
			NextNum: 2,
		},
	}
}
