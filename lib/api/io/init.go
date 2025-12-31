package io

import (
	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/io"
	"github.com/gofiber/fiber/v2"
)

func Init(store *lib.InitStore) {
	exportEtherpad := io.NewExportEtherpad(store.Hooks, store.PadManager, store.Store, store.Logger, store.UiAssets)
	store.C.Get("/p/:pad/:rev/export/:type", func(ctx *fiber.Ctx) error {
		return GetExport(ctx, exportEtherpad, store.RetrievedSettings, store.Logger, store.PadManager, store.ReadOnlyManager, store.SecurityManager)
	})
	store.C.Get("/p/:pad/export/:type", func(ctx *fiber.Ctx) error {
		return GetExport(ctx, exportEtherpad, store.RetrievedSettings, store.Logger, store.PadManager, store.ReadOnlyManager, store.SecurityManager)
	})
}
