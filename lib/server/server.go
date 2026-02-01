package server

import (
	"embed"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/ether/etherpad-go/lib"
	api2 "github.com/ether/etherpad-go/lib/api"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/io"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/plugins"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
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

	padManager := pad.NewManager(dataStore, &retrievedHooks)
	authorManager := author.NewManager(dataStore)
	importer := io.NewImporter(padManager, authorManager, dataStore, setupLogger)
	globalHub := ws.NewHub()
	sessionStore := ws.NewSessionStore()
	padMessageHandler := ws.NewPadMessageHandler(dataStore, &retrievedHooks, padManager, &sessionStore, globalHub, setupLogger)
	adminMessageHandler := ws.NewAdminMessageHandler(dataStore, &retrievedHooks, padManager, padMessageHandler, setupLogger, globalHub, app)
	securityManager := pad.NewSecurityManager(dataStore, &retrievedHooks, padManager)

	var epPluginStore = &interfaces.EpPluginStore{
		Logger:            setupLogger,
		HookSystem:        &retrievedHooks,
		UIAssets:          uiAssets,
		PadManager:        padManager,
		App:               app,
		RetrievedSettings: &settings,
	}

	// init plugins
	plugins.InitPlugins(epPluginStore)

	var cookieStore = session.New(session.Config{
		KeyLookup:      "cookie:express_sid",
		Storage:        db,
		CookieSameSite: settings.Cookie.SameSite,
		Expiration:     time.Duration(settings.Cookie.SessionLifetime),
	})

	hooks.ExpressPreSession(app, uiAssets)
	go globalHub.Run()

	if err != nil {
		setupLogger.Fatal("Error connecting to database: " + err.Error())
		return
	}

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
		Importer:          importer,
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
				if err != nil {
					setupLogger.Warn("Invalid token provided for admin validation: " + err.Error())
				} else {
					setupLogger.Warn("Invalid token provided for admin validation")
				}
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
