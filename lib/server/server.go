package server

import (
	"context"
	"embed"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib"
	api2 "github.com/ether/etherpad-go/lib/api"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins"
	session2 "github.com/ether/etherpad-go/lib/session"
	settings2 "github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/adaptor/v2"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/session"
	"go.uber.org/zap"
)

func InitServer(setupLogger *zap.SugaredLogger, uiAssets embed.FS) {

	settings2.InitSettings(setupLogger)

	var settings = settings2.Displayed
	validatorEvaluator := validator.New(validator.WithRequiredStructEnabled())

	retrievedHooks := hooks.NewHook()

	// init plugins
	plugins.InitPlugins(&settings, &retrievedHooks, setupLogger, uiAssets)

	gitVersion := settings2.GetGitCommit(&settings)
	setupLogger.Info("Starting Etherpad Go...")
	setupLogger.Info("Report bugs at https://github.com/ether/etherpad-go/issues")
	setupLogger.Info("Your Etherpad Go version is " + gitVersion)
	settings.GitVersion = gitVersion

	dataStore, err := utils.GetDB(settings, setupLogger)
	readOnlyManager := pad.NewReadOnlyManager(dataStore)

	var db = session2.NewSessionDatabase(nil)
	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
	})

	app.Use(func(c *fiber.Ctx) error {
		return pad.CheckAccess(c, setupLogger, &settings, readOnlyManager)
	})

	var cookieStore = session.New(session.Config{
		KeyLookup:      "cookie:express_sid",
		Storage:        db,
		CookieSameSite: settings.Cookie.SameSite,
		Expiration:     time.Duration(settings.Cookie.SessionLifetime),
	})

	hooks.ExpressPreSession(app, uiAssets)
	globalHub := ws.NewHub()
	go globalHub.Run()

	sessionStore := ws.NewSessionStore()

	if err != nil {
		setupLogger.Fatal("Error connecting to database: " + err.Error())
		return
	}

	padManager := pad.NewManager(dataStore, &retrievedHooks)
	authorManager := author.NewManager(dataStore)

	padMessageHandler := ws.NewPadMessageHandler(dataStore, &retrievedHooks, padManager, &sessionStore, globalHub, setupLogger)
	adminMessageHandler := ws.NewAdminMessageHandler(dataStore, &retrievedHooks, padManager, padMessageHandler, setupLogger, globalHub)
	securityManager := pad.NewSecurityManager(dataStore, &retrievedHooks, padManager)
	adminAPIRoute := app.Group("/admin/api")

	authenticator := api2.InitAPI(&lib.InitStore{
		C:                 app,
		PrivateAPI:        adminAPIRoute,
		Validator:         validatorEvaluator,
		PadManager:        padManager,
		AuthorManager:     authorManager,
		Hooks:             &retrievedHooks,
		RetrievedSettings: &settings,
		Logger:            setupLogger,
		SecurityManager:   securityManager,
		UiAssets:          uiAssets,
		CookieStore:       cookieStore,
		Handler:           padMessageHandler,
		Store:             dataStore,
		ReadOnlyManager:   readOnlyManager,
	})

	app.Get("/socket.io/*", func(c *fiber.Ctx) error {
		return adaptor.HTTPHandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			ws.ServeWs(writer, request, cookieStore, c, &settings, setupLogger, padMessageHandler)
		})(c)
	})

	ssoAdminClient := settings.SSO.GetAdminClient()

	if ssoAdminClient != nil {
		app.Get("/admin/validate", func(c *fiber.Ctx) error {
			token := c.Query("token")
			if token == "" {
				setupLogger.Info("No token provided for admin validation")
				return c.Status(http.StatusUnauthorized).Send([]byte("No token provided"))
			}
			ok, err := authenticator.ValidateAdminToken(token, ssoAdminClient)
			if err != nil || !ok {
				setupLogger.Warn("Invalid token provided for admin validation: " + err.Error())
				return c.Status(http.StatusUnauthorized).Send([]byte("No token provided"))
			}
			return c.SendStatus(http.StatusOK)
		})

		app.Get("/admin/ws", func(c *fiber.Ctx) error {
			token := c.Query("token")
			if token == "" {
				setupLogger.Warn("No token provided for websocket connection")
				return c.Status(http.StatusUnauthorized).Send([]byte("No token provided"))
			}
			ok, err := authenticator.ValidateAdminToken(token, ssoAdminClient)
			if err != nil || !ok {
				setupLogger.Warn("Invalid token provided for websocket connection: " + err.Error())
				return c.Status(http.StatusUnauthorized).Send([]byte("No token provided"))
			}
			return adaptor.HTTPHandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
				ws.ServeAdminWs(writer, request, c, &settings, setupLogger, adminMessageHandler)
			})(c)
		})
	}

	fiberString := fmt.Sprintf("%s:%s", settings.IP, settings.Port)
	setupLogger.Info("Starting Web UI on " + fiberString)
	err = app.Listen(fiberString)
	if err != nil {
		setupLogger.Error("Error starting web UI: " + err.Error())
		os.Exit(1)
	}
}
