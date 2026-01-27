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

// @title Fiber Example API
// @version 1.0
// @description This is a sample swagger for Fiber
// @termsOfService http://swagger.io/terms/
// @contact.name API Support
// @contact.email fiber@swagger.io
// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html
// @host localhost:3000
// @BasePath /
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
			server.PrepareServer(setupLogger)
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
	server := server2.New()

	server.Run(setupLogger, uiAssets)
}
