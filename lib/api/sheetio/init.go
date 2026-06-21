package sheetio

import "github.com/ether/etherpad-go/lib"

// Init registers the spreadsheet xlsx import/export routes.
func Init(store *lib.InitStore) {
	store.C.Post("/s/:pad/import", ImportSheet(store))
	store.C.Get("/s/:pad/export.xlsx", ExportSheet(store))
}
