package test

import (
	"fmt"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ether/etherpad-go/lib"
	// "github.com/ether/etherpad-go/lib/api"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/cli"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/loadtest"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/test/testutils"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"go.uber.org/zap"
)

func TestIntegration(t *testing.T) {
	// Setup environment for testing
	os.Setenv("GO_TEST_MODE", "true")
	defer os.Unsetenv("GO_TEST_MODE")

	logger := zap.NewNop().Sugar()

	// Use Memory DataStore for integration test for speed
	dataStore := db.NewMemoryDataStore()
	defer dataStore.Close()

	hook := hooks.NewHook()
	hub := ws.NewHub()
	go hub.Run()

	sessionStore := ws.NewSessionStore()
	padManager := pad.NewManager(dataStore, &hook)
	authorManager := author.NewManager(dataStore)
	padMessageHandler := ws.NewPadMessageHandler(dataStore, &hook, padManager, &sessionStore, hub, logger)
	securityManager := pad.NewSecurityManager(dataStore, &hook, padManager)
	readOnlyManager := pad.NewReadOnlyManager(dataStore)

	settings.Displayed = settings.Settings{
		IP:   "127.0.0.1",
		Port: "3000",
		SSO: &settings.SSO{
			Issuer: "http://localhost:3000",
		},
	}

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(func(c *fiber.Ctx) error {
		fmt.Printf("DEBUG Request: %s %s, Content-Type: %s\n", c.Method(), c.Path(), c.Get("Content-Type"))
		err := c.Next()
		if err != nil {
			fmt.Printf("DEBUG Error in route: %v\n", err)
		}
		return err
	})

	// Setup session middleware
	cookieStore := session.New(session.Config{
		KeyLookup: "cookie:express_sid",
	})

	// Setup API and WebSocket routes similar to main.go
	_ = &lib.InitStore{
		C:                 app,
		Validator:         validator.New(validator.WithRequiredStructEnabled()),
		PadManager:        padManager,
		AuthorManager:     authorManager,
		Hooks:             &hook,
		Logger:            logger,
		SecurityManager:   securityManager,
		UiAssets:          testutils.GetTestAssets(),
		CookieStore:       cookieStore,
		Handler:           padMessageHandler,
		Store:             dataStore,
		ReadOnlyManager:   readOnlyManager,
		RetrievedSettings: &settings.Displayed,
	}
	// api.InitAPI(libStore)

	app.Get("/p/:padId", func(c *fiber.Ctx) error {
		padID := c.Params("padId")
		fmt.Printf("Accessing pad: %s\n", padID)
		// Ensure pad exists in manager
		p, _ := padManager.GetPad(padID, nil, nil)
		p.SetText("Initial Text\n", nil)
		c.Set("Content-Type", "text/html")
		return c.SendString("<html><body>Pad</body></html>")
	})

	app.Get("/socket.io/*", func(c *fiber.Ctx) error {
		c.Locals("ctx", c)
		return ws.ServeWsFiber(cookieStore, &settings.Displayed, logger, padMessageHandler)(c)
	})

	// Start test server
	ts := httptest.NewServer(adaptor.FiberApp(app))
	defer ts.Close()

	settings.Displayed.SSO.Issuer = ts.URL

	t.Run("CLI_Append_Verification", func(t *testing.T) {
		padID := "test-pad-" + time.Now().Format("150405")
		host := fmt.Sprintf("%s/p/%s", ts.URL, padID)
		appendStr := "Hello from Integration Test"

		// Run CLI Append
		cli.RunFromCLI(logger, []string{"-host", host, "-append", appendStr})

		// Verify via PadManager
		p, err := padManager.GetPad(padID, nil, nil)
		if err != nil {
			t.Fatalf("Failed to get pad: %v", err)
		}

		text := p.Text()
		if !strings.Contains(text, appendStr) {
			t.Errorf("Expected pad to contain %q, but got %q", appendStr, text)
		}
	})

	t.Run("Loadtest_Short_Run", func(t *testing.T) {
		padID := "load-test-pad"
		host := fmt.Sprintf("%s/p/%s", ts.URL, padID)

		// Run a very short loadtest (1 second)
		loadtest.RunFromCLI(logger, []string{"-host", host, "-authors", "1", "-duration", "1"})

		// If it doesn't panic or hang, we consider it a success for this integration test
		if _, err := padManager.GetPad(padID, nil, nil); err == nil {
			// In Memory store might not have revisions if not saved properly but let's check
			// We just want to see if the loadtest ran without errors
			t.Logf("Loadtest finished for pad %s", padID)
		}
	})
}
