package server

import (
	"os"
	"os/exec"

	"go.uber.org/zap"
)

type RequiredTool struct {
	Name           string
	Description    string
	UrlForDownload string
}

var requiredTools = []RequiredTool{
	{
		Name:           "node",
		Description:    "Node.js is required to run Etherpad Go server.",
		UrlForDownload: "https://nodejs.org/",
	},
	{
		Name:           "pnpm",
		Description:    "pnpm is a fast, disk space efficient package manager.",
		UrlForDownload: "https://pnpm.io/installation",
	},
}

func PrepareServer(setupLogger *zap.SugaredLogger) {
	check := func(rqT RequiredTool) {
		path, err := exec.LookPath(rqT.Name)
		if err != nil {
			setupLogger.Warnf("%s: NOT found in PATH\n", rqT.Name)
			setupLogger.Warnf("Description: %s\n", rqT.Description)
			setupLogger.Warnf("Please download and install it from: %s\n", rqT.UrlForDownload)

			os.Exit(1)
		}
		setupLogger.Infof("%s: found at %s\n ✅", rqT.Name, path)
	}
	for _, tool := range requiredTools {
		check(tool)
	}
	buildUI(setupLogger)
	buildServer(setupLogger)

	setupLogger.Infof("Building the ui completed successfully.")
	setupLogger.Infof("Next the actual server is compiled with the ui assets embedded.")

	setupLogger.Infof("Build completed successfully. You ")
}

func buildUI(setupLogger *zap.SugaredLogger) {
	cmd := exec.Command("node", "./build.js")
	cmd.Dir = "./ui"

	// Streamt direkt in die aktuelle Console
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		setupLogger.Fatalf("build failed: %v", err)
	}
}

func buildServer(setupLogger *zap.SugaredLogger) {
	exeName := "etherpad"

	if os.Getenv("GOOS") == "windows" {
		exeName += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", exeName, ".")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		setupLogger.Fatalf("build failed: %v", err)
	}
}
