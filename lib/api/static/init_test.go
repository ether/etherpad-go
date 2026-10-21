package static

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/evanw/esbuild/pkg/api"
)

func TestExternalFontsPluginKeepsFontURLsExternal(t *testing.T) {
	dir := t.TempDir()
	cssPath := filepath.Join(dir, "style.css")
	css := `@font-face { font-family: test; src: url("fontawesome.woff?2") format("woff"); }
body { background: url("icon.svg"); }`
	if err := os.WriteFile(cssPath, []byte(css), 0o600); err != nil {
		t.Fatalf("write test stylesheet: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "icon.svg"), []byte("<svg/>"), 0o600); err != nil {
		t.Fatalf("write test image: %v", err)
	}

	result := api.Build(api.BuildOptions{
		EntryPoints:   []string{"style.css"},
		AbsWorkingDir: dir,
		Bundle:        true,
		Write:         false,
		Loader:        loaderConfig,
		Plugins:       []api.Plugin{externalFonts},
	})
	if len(result.Errors) > 0 {
		t.Fatalf("build stylesheet: %v", result.Errors)
	}
	if len(result.OutputFiles) != 1 {
		t.Fatalf("expected one output file, got %d", len(result.OutputFiles))
	}

	output := string(result.OutputFiles[0].Contents)
	if !strings.Contains(output, "fontawesome.woff?2") {
		t.Fatalf("font URL was not preserved: %s", output)
	}
	if strings.Contains(output, "data:font/") {
		t.Fatalf("font URL was inlined: %s", output)
	}
	if !strings.Contains(output, "data:image/svg+xml") {
		t.Fatalf("non-font asset should still be inlined: %s", output)
	}
}
