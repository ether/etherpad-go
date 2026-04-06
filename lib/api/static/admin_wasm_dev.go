package static

import (
	"embed"
	"os"
	"path/filepath"

	"github.com/ether/etherpad-go/lib/settings"
	"github.com/gofiber/fiber/v3"
)

func serveAdminAsset(c fiber.Ctx, uiAssets embed.FS, retrievedSettings *settings.Settings, assetName string, contentType string) error {
	if isDevEnabled(retrievedSettings) {
		fileContent, err := os.ReadFile(filepath.Join(retrievedSettings.Root, "assets", "js", "admin", assetName))
		if err == nil {
			if contentType != "" {
				c.Set("Content-Type", contentType)
			}
			return c.Send(fileContent)
		}
	}
	fileContent, err := uiAssets.ReadFile(filepath.ToSlash(filepath.Join("assets", "js", "admin", assetName)))
	if err != nil {
		return err
	}
	if contentType != "" {
		c.Set("Content-Type", contentType)
	}
	return c.Send(fileContent)
}
