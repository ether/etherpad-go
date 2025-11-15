package testutils

import (
	db2 "github.com/ether/etherpad-go/lib/db"
	hooks2 "github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/ws"
)

func InitMemoryUtils() (*db2.MemoryDataStore, *hooks2.Hook, *pad.Manager, *ws.PadMessageHandler) {
	db := db2.NewMemoryDataStore()
	hooks := hooks2.NewHook()
	manager := pad.NewManager(db, &hooks)
	padHandler := ws.NewPadMessageHandler(db, &hooks, &manager)

	return db, &hooks, &manager, padHandler
}
