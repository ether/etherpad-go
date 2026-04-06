package static

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	adminui "github.com/ether/etherpad-go/assets/admin"
	"github.com/ether/etherpad-go/lib"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/fsnotify/fsnotify"
	"github.com/gofiber/fiber/v3"
)

type adminWASMDevBuilder struct {
	root   string
	logger interface {
		Infof(string, ...interface{})
		Warnf(string, ...interface{})
		Errorf(string, ...interface{})
	}
	mu sync.Mutex
}

func calcAdminDataConfig(retrievedSettings *settings.Settings) string {
	if retrievedSettings == nil || retrievedSettings.SSO == nil {
		return ""
	}
	for _, client := range retrievedSettings.SSO.Clients {
		if client.Type == "admin" {
			if retrievedSettings.SSO.Issuer == "" || len(client.RedirectUris) == 0 {
				return ""
			}
			selectedClient := client
			oidcConfig := settings.OidcConfig{
				ClientId:    selectedClient.ClientId,
				Authority:   retrievedSettings.SSO.Issuer,
				JwksUri:     retrievedSettings.SSO.Issuer + ".well-known/jwks.json",
				RedirectUri: selectedClient.RedirectUris[0],
				Scope:       []string{"openid", "profile", "email", "offline"},
			}
			conf, _ := json.Marshal(oidcConfig)
			return string(conf)
		}
	}
	return ""
}

func getAdminBody(_ embed.FS, retrievedSettings *settings.Settings) (*string, error) {
	component := adminui.Admin(*retrievedSettings, calcAdminDataConfig(retrievedSettings))
	var buf bytes.Buffer
	if err := component.Render(context.Background(), &buf); err != nil {
		return nil, err
	}
	result := buf.String()
	return &result, nil
}

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

func ensureAdminSupportFiles(root string) error {
	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		out, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			return err
		}
		goroot = string(bytes.TrimSpace(out))
	}

	targetDir := filepath.Join(root, "assets", "js", "admin")
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return err
	}

	source := filepath.Join(goroot, "lib", "wasm", "wasm_exec.js")
	target := filepath.Join(targetDir, "wasm_exec.js")
	input, err := os.ReadFile(source)
	if err != nil {
		return err
	}
	return os.WriteFile(target, input, 0o644)
}

func (b *adminWASMDevBuilder) build() error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if err := ensureAdminSupportFiles(b.root); err != nil {
		return err
	}

	outPath := filepath.Join(b.root, "assets", "js", "admin", "admin.wasm")
	cmd := exec.Command("go", "build", "-o", outPath, "./admin/wasm")
	cmd.Dir = b.root
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("admin wasm build failed: %w: %s", err, stderr.String())
	}
	b.logger.Infof("admin wasm build complete")
	return nil
}

func startAdminWASMDevBuilder(store *lib.InitStore) (*adminWASMDevBuilder, error) {
	if !isDevEnabled(store.RetrievedSettings) {
		return nil, nil
	}
	builder := &adminWASMDevBuilder{root: store.RetrievedSettings.Root, logger: store.Logger}
	if err := builder.build(); err != nil {
		return nil, err
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	watchPath := filepath.Join(store.RetrievedSettings.Root, "admin", "wasm")
	if err := watcher.Add(watchPath); err != nil {
		watcher.Close()
		return nil, err
	}

	go func() {
		defer watcher.Close()
		var timerMu sync.Mutex
		var timer *time.Timer
		schedule := func() {
			timerMu.Lock()
			defer timerMu.Unlock()
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(250*time.Millisecond, func() {
				if err := builder.build(); err != nil {
					store.Logger.Errorf("admin wasm rebuild failed: %v", err)
				}
			})
		}
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) != 0 {
					schedule()
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				store.Logger.Warnf("admin wasm watcher error: %v", err)
			}
		}
	}()
	store.Logger.Infof("admin wasm dev watch enabled")
	return builder, nil
}
