package io

import (
	"slices"

	"github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

func GetExport(ctx *fiber.Ctx, exportHandler *io.ExportEtherpad, settings *settings.Settings, logger *zap.SugaredLogger, padManager *pad.Manager, readOnlyManager *pad.ReadOnlyManager, securityManager *pad.SecurityManager) error {
	padId := ctx.Params("pad")
	rev := ctx.Params("rev")
	exportType := ctx.Params("type")
	typesToExport := []string{
		"pdf", "doc", "docx", "txt", "html", "odt", "etherpad",
	}
	externalTypes := []string{
		"odt",
	}

	if !slices.Contains(typesToExport, exportType) {
		return ctx.Status(400).SendString("Invalid export type")
	}

	if settings.ExportToExternalToolsAvailable() == "no" && slices.Contains(externalTypes, exportType) {
		logger.Warnf("Export to %s requested but exporting is disabled in settings", exportType)
		return ctx.Status(503).SendString("Exporting to " + exportType + " is not available")
	}
	ctx.Response().Header.Set("Access-Control-Allow-Origin", "*")

	if securityManager.HasPadAccess(ctx) {
		var readOnlyId *string = nil
		if readOnlyManager.IsReadOnlyID(&padId) {
			readOnlyId = &padId
			actualPadId, err := readOnlyManager.GetPadId(padId)
			if err != nil {
				return ctx.Status(404).SendString("Pad not found")
			}
			padId = *actualPadId
		}

		exists, err := padManager.DoesPadExist(padId)
		if err != nil {
			return ctx.Status(500).SendString("Internal server error")
		}
		if !*exists {
			return ctx.Status(404).SendString("Pad not found")
		}

		logger.Infof("Exporting pad %s revision %s to %s", padId, rev, exportType)

		return exportHandler.DoExport(ctx, padId, readOnlyId, exportType)
	}

	return ctx.Status(401).SendString("Unauthorized to access this pad")
}
