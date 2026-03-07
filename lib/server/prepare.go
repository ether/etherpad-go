package server

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

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

	execPath, err := os.Getwd()
	if err != nil {
		setupLogger.Fatalf("Could not determine executable path: %v", err)
	}

	installDeps(setupLogger, filepath.Join(execPath, "ui"))

	buildUI(setupLogger)
	buildAdminWASM(setupLogger, execPath)
	serverPath := buildServer(setupLogger)

	setupLogger.Infof("Building the ui completed successfully.")
	setupLogger.Infof("Next the actual server is compiled with the ui assets embedded.")

	setupLogger.Infof("Build completed successfully. You can find the executable at %s", serverPath)
}

func installDeps(setupLogger *zap.SugaredLogger, pathToInstall string) {
	cmd := exec.Command("pnpm", "install")
	cmd.Dir = pathToInstall

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		setupLogger.Fatalf("dependency installation failed: %v", err)
	}
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

func buildAdminWASM(setupLogger *zap.SugaredLogger, root string) {
	cmd := exec.Command("go", "build", "-o", filepath.Join(root, "assets", "js", "admin", "admin.wasm"), "./admin/wasm")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := os.MkdirAll(filepath.Join(root, "assets", "js", "admin"), 0o755); err != nil {
		setupLogger.Fatalf("failed to create admin wasm output dir: %v", err)
	}

	goroot := os.Getenv("GOROOT")
	if goroot == "" {
		out, err := exec.Command("go", "env", "GOROOT").Output()
		if err != nil {
			setupLogger.Fatalf("failed to resolve GOROOT: %v", err)
		}
		goroot = strings.TrimSpace(string(out))
	}
	wasmExecBytes, err := os.ReadFile(filepath.Join(goroot, "lib", "wasm", "wasm_exec.js"))
	if err != nil {
		setupLogger.Fatalf("failed to read wasm_exec.js: %v", err)
	}
	if err := os.WriteFile(filepath.Join(root, "assets", "js", "admin", "wasm_exec.js"), wasmExecBytes, 0o644); err != nil {
		setupLogger.Fatalf("failed to write wasm_exec.js: %v", err)
	}

	if err := cmd.Run(); err != nil {
		setupLogger.Fatalf("admin wasm build failed: %v", err)
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
