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
	settings2 "github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/ether/etherpad-go/lib/ws"
	"github.com/go-playground/validator/v10"
	fiberws "github.com/gofiber/contrib/v3/websocket"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/compress"
	"github.com/gofiber/fiber/v3/middleware/session"
	"go.uber.org/zap"
)

func InitServer(setupLogger *zap.SugaredLogger, uiAssets embed.FS, pluginAssets embed.FS) {

	settings2.InitSettings(setupLogger)
	plugins.Init(uiAssets, pluginAssets)

	var settings = settings2.Displayed
	validatorEvaluator := validator.New(validator.WithRequiredStructEnabled())

	retrievedHooks := hooks.NewHook()

	gitVersion := settings2.GetGitCommit(&settings)
	setupLogger.Info("Starting Etherpad Go...")
	setupLogger.Info("Report bugs at https://github.com/ether/etherpad-go/issues")
	setupLogger.Info("Your Etherpad Go version is " + gitVersion)
	settings.GitVersion = gitVersion

	dataStore, err := utils.GetDB(settings, setupLogger)
	if err != nil {
		setupLogger.Fatal("Error connecting to database: " + err.Error())
		return
	}

	StartUpdateRoutine(setupLogger, dataStore, gitVersion)

	readOnlyManager := pad.NewReadOnlyManager(dataStore)

	app := fiber.New(fiber.Config{})
	app.Use(compress.New(compress.Config{
		Level: compress.LevelBestSpeed,
	}))

	app.Use(func(c fiber.Ctx) error {
		return pad.CheckAccess(c, setupLogger, &settings, readOnlyManager)
	})

	padManager := pad.NewManager(dataStore, &retrievedHooks)
	authorManager := author.NewManager(dataStore)
	importer := io.NewImporter(padManager, authorManager, dataStore, setupLogger)
	globalHub := ws.NewHub()
	sessionStore := ws.NewSessionStore()
	padMessageHandler := ws.NewPadMessageHandler(dataStore, &retrievedHooks, padManager, &sessionStore, globalHub, setupLogger, uiAssets)
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

	var cookieStore = session.NewStore(session.Config{
		CookieSameSite: settings.Cookie.SameSite,
		IdleTimeout:    time.Duration(settings.Cookie.SessionLifetime),
	})

	hooks.ExpressPreSession(app, uiAssets)
	go globalHub.Run()

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

	app.Use("/socket.io", func(c fiber.Ctx) error {
		if fiberws.IsWebSocketUpgrade(c) {
			sess, err := cookieStore.Get(c)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("session error")
			}
			c.Locals("sessionID", sess.ID())
			c.Locals("clientIP", c.IP())
			// Preserve web access user info set by CheckAccess middleware
			c.Locals("webAccessUser", c.Locals("sessionUser"))
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get("/socket.io/*", fiberws.New(func(conn *fiberws.Conn) {
		sessionID, _ := conn.Locals("sessionID").(string)
		clientIP, _ := conn.Locals("clientIP").(string)
		webAccessUser := conn.Locals("webAccessUser")
		ws.ServeWs(conn, sessionID, clientIP, webAccessUser, &settings, setupLogger, padMessageHandler)
	}, fiberws.Config{
		ReadBufferSize:  1024,
		WriteBufferSize: 1024,
		Origins:         []string{"*"},
	}))

	ssoAdminClient := settings.SSO.GetAdminClient()

	if ssoAdminClient != nil {
		app.Get("/admin/validate", func(c fiber.Ctx) error {
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

		app.Use("/admin/ws", func(c fiber.Ctx) error {
			if fiberws.IsWebSocketUpgrade(c) {
				token := c.Query("token")
				if token == "" {
					setupLogger.Warn("No token provided for websocket connection")
					return c.Status(http.StatusUnauthorized).Send([]byte("No token provided"))
				}
				ok, err := authenticator.ValidateAdminToken(token, ssoAdminClient)
				if err != nil || !ok {
					setupLogger.Warn("Invalid token provided for websocket connection")
					return c.Status(http.StatusUnauthorized).Send([]byte("No token provided"))
				}
				c.Locals("fiberCtx", c)
				return c.Next()
			}
			return fiber.ErrUpgradeRequired
		})
		app.Get("/admin/ws", fiberws.New(func(conn *fiberws.Conn) {
			ws.ServeAdminWs(conn, &settings, setupLogger, adminMessageHandler)
		}, fiberws.Config{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
			Origins:         []string{"*"},
		}))
	}

	fiberString := fmt.Sprintf("%s:%s", settings.IP, settings.Port)
	setupLogger.Info("Starting Web UI on " + fiberString)
	err = app.Listen(fiberString, fiber.ListenConfig{DisableStartupMessage: true})
	if err != nil {
		setupLogger.Error("Error starting web UI: " + err.Error())
		os.Exit(1)
	}
}
