package pad

import (
	"github.com/brianvoe/gofakeit/v7"
	"github.com/ether/etherpad-go/lib/db"
	hooks2 "github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/models/pad"
)

func CreateNewPad(ds db.DataStore) *pad.Pad {
	hooksToInject := hooks2.NewHook()
	createdPad := pad.NewPad(gofakeit.Name(), ds, &hooksToInject)
	return &createdPad
}
