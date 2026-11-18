package sheetio

import "github.com/ether/etherpad-go/lib"

// Init registers the spreadsheet xlsx import/export and history routes.
func Init(store *lib.InitStore) {
	store.C.Post("/s/:pad/import", ImportSheet(store))
	store.C.Get("/s/:pad/export.xlsx", ExportSheet(store))
	store.C.Get("/s/:pad/history", SheetHistory(store))
	store.C.Get("/s/:pad/history/:rev", SheetAtRevision(store))
	store.C.Post("/s/:pad/history/:rev/restore", RestoreSheet(store))
}
