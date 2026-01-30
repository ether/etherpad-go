package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

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
	serverPath := buildServer(setupLogger)

	setupLogger.Infof("Building the ui completed successfully.")
	setupLogger.Infof("Next the actual server is compiled with the ui assets embedded.")

	setupLogger.Infof("Build completed successfully. You can find the executable at %s", serverPath)
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

func buildServer(setupLogger *zap.SugaredLogger) string {
	exeName := "etherpad"

	if runtime.GOOS == "windows" {
		exeName += ".exe"
	}
	exePath, err := os.Executable()

	etherpadPath := filepath.Join(filepath.Dir(exePath), exeName)

	fileInfo, err := os.Stat(etherpadPath)

	setupLogger.Infof("The binary will be built at %s", etherpadPath)
	if err != nil && !os.IsNotExist(err) {
		setupLogger.Infof("Error accessing %s", exePath)
	}

	if err == nil && fileInfo.Mode().IsRegular() {
		setupLogger.Info("Old etherpad executable found, removing it before building a new one.")
		err = os.Remove(etherpadPath)
		if err != nil {
			setupLogger.Fatalf("Could not remove old etherpad executable: %v", err)
		}
	}

	cmd := exec.Command("go", "build", "-o", etherpadPath, ".")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		setupLogger.Fatalf("build failed: %v", err)
	}

	return etherpadPath
}
