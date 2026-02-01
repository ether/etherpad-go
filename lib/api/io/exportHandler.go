package io

import (
	"slices"

	"github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"
)

// GetExport godoc
// @Summary Export a pad
// @Description Exports the content of a pad to various formats (pdf, word, txt, html, open, etherpad, markdown)
// @Tags Export
// @Produce octet-stream
// @Param pad path string true "Pad ID"
// @Param rev path string false "Revision number"
// @Param type path string true "Export type (pdf, word, txt, html, open, etherpad, markdown)"
// @Success 200 {file} binary "Exported file"
// @Failure 400 {string} string "Invalid export type"
// @Failure 401 {string} string "Unauthorized"
// @Failure 404 {string} string "Pad not found"
// @Failure 500 {string} string "Internal server error"
// @Failure 503 {string} string "Export not available"
// @Router /p/{pad}/export/{type} [get]
// @Router /p/{pad}/{rev}/export/{type} [get]
func GetExport(ctx *fiber.Ctx, exportHandler *io.ExportEtherpad, settings *settings.Settings, logger *zap.SugaredLogger, padManager *pad.Manager, readOnlyManager *pad.ReadOnlyManager, securityManager *pad.SecurityManager) error {
	padId := ctx.Params("pad")
	rev := ctx.Params("rev")
	exportType := ctx.Params("type")
	typesToExport := map[string]string{
		"pdf":      "pdf",
		"word":     "docx",
		"txt":      "txt",
		"html":     "html",
		"open":     "odt",
		"etherpad": "etherpad",
		"markdown": "md",
	}
	// All formats are now supported internally, no external tools needed
	var externalTypes []string

	if _, ok := typesToExport[exportType]; !ok {
		return ctx.Status(400).SendString("Invalid export type")
	}

	if slices.Contains(externalTypes, exportType) {
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

		return exportHandler.DoExport(ctx, padId, readOnlyId, typesToExport[exportType])
	}

	return ctx.Status(401).SendString("Unauthorized to access this pad")
}
