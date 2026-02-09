package main

import (
	"embed"
	"fmt"
	_ "fmt"
	"os"

	_ "github.com/ether/etherpad-go/docs"
	"github.com/ether/etherpad-go/lib/cli"
	"github.com/ether/etherpad-go/lib/loadtest"
	"github.com/ether/etherpad-go/lib/locales"
	"github.com/ether/etherpad-go/lib/migration"
	server2 "github.com/ether/etherpad-go/lib/server"
	settings2 "github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
)

//go:embed assets
var uiAssets embed.FS

//go:embed plugins
var pluginAssets embed.FS

// @title Etherpad Go API
// @version 1.0
// @description REST API for Etherpad Go - Collaborative Text Editor
// @termsOfService http://swagger.io/terms/

// @contact.name Etherpad Support
// @contact.url https://github.com/ether/etherpad-go
// @contact.email support@etherpad.org

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @host localhost:9001
// @BasePath /

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Bearer Token for Admin API authentication

func main() {
	setupLogger := utils.SetupLogger()
	defer setupLogger.Sync()

	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "migration":
			migration.RunFromCLI(setupLogger, os.Args[2:])
			return
		case "cli":
			cli.RunFromCLI(setupLogger, os.Args[2:])
			return
		case "loadtest":
			loadtest.RunFromCLI(setupLogger, os.Args[2:])
			return
		case "multiload":
			loadtest.RunMultiFromCLI(setupLogger, os.Args[2:])
			return
		case "prepare":
			server2.PrepareServer(setupLogger)
			os.Exit(0)
		case "config":
			settings2.HandleConfigCommand(setupLogger)
		case "locales":
			locales.Handle()
		case "-h", "--help", "help":
			fmt.Println("Usage: etherpad [command] [options]")
			fmt.Println("Commands:")
			fmt.Println("  cli        Interactive CLI for pads")
			fmt.Println("  loadtest   Run a load test on a single pad")
			fmt.Println("  multiload  Run a multi-pad load test")
			fmt.Println("  (none)     Start the Etherpad server")
			fmt.Println("  prepare    Prepare the etherpad server including building frontend assets")
			fmt.Println(" config     Manage configuration settings")
			return
		}
	}

	server2.InitServer(setupLogger, uiAssets, pluginAssets)
}
