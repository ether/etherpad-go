package testutils

import (
	"github.com/ether/etherpad-go/lib/author"
	db2 "github.com/ether/etherpad-go/lib/db"
	hooks2 "github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/go-playground/validator/v10"
)

type TestUtilFields struct {
	DB                *db2.MemoryDataStore
	Hooks             *hooks2.Hook
	PadManager        *pad.Manager
	PadMessageHandler *ws.PadMessageHandler
	Validator         *validator.Validate
	AuthorManager     *author.Manager
}

func InitMemoryUtils() *TestUtilFields {
	db := db2.NewMemoryDataStore()
	hooks := hooks2.NewHook()
	manager := pad.NewManager(db, &hooks)
	managerAuthor := author.NewManager(db)
	padHandler := ws.NewPadMessageHandler(db, &hooks, manager)
	validatorEvaluator := validator.New(validator.WithRequiredStructEnabled())

	return &TestUtilFields{
		DB:                db,
		Hooks:             &hooks,
		PadManager:        manager,
		PadMessageHandler: padHandler,
		Validator:         validatorEvaluator,
		AuthorManager:     managerAuthor,
	}
}
