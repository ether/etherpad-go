package static

import (
	"encoding/json"
	"path"
	"strings"
	"sync"

	"github.com/ether/etherpad-go/lib"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/gofiber/fiber/v3"
)

type devBundleState struct {
	name       string
	entryPoint string
	ctx        api.BuildContext
	mu         sync.RWMutex
	output     []byte
	ready      bool
}

type esbuildDevHMR struct {
	logger interface {
		Infof(string, ...interface{})
		Warnf(string, ...interface{})
	}
	notify  func(string)
	bundles map[string]*devBundleState
}

func (h *esbuildDevHMR) bundleFromPath(requestPath string) *devBundleState {
	switch {
	case strings.Contains(requestPath, "welcome"):
		return h.bundles["welcome"]
	case strings.Contains(requestPath, "timeslider"):
		return h.bundles["timeslider"]
	default:
		return h.bundles["pad"]
	}
}

func (h *esbuildDevHMR) serveBundle(c fiber.Ctx) error {
	bundle := h.bundleFromPath(c.Path())
	if bundle == nil {
		return c.Status(fiber.StatusNotFound).SendString("Unknown JS bundle")
	}

	bundle.mu.RLock()
	output := append([]byte(nil), bundle.output...)
	ready := bundle.ready
	bundle.mu.RUnlock()

	if !ready || len(output) == 0 {
		result := bundle.ctx.Rebuild()
		if len(result.Errors) > 0 || len(result.OutputFiles) == 0 {
			return c.Status(fiber.StatusInternalServerError).SendString("esbuild rebuild failed")
		}
		output = result.OutputFiles[0].Contents
		bundle.mu.Lock()
		bundle.output = output
		bundle.ready = true
		bundle.mu.Unlock()
	}

	c.Set("Content-Type", "application/javascript")
	return c.Send(output)
}

func buildDevAliases() map[string]string {
	relativePath := "./src/js"
	return map[string]string{
		"ep_etherpad-lite/static/js/ace2_inner":              relativePath + "/ace2_inner",
		"ep_etherpad-lite/static/js/ace2_common":             relativePath + "/ace2_common",
		"ep_etherpad-lite/static/js/pad_cookie":              relativePath + "/pad_cookie",
		"ep_etherpad-lite/static/js/pluginfw/client_plugins": relativePath + "/pluginfw/client_plugins",
	}
}

func newDevBundle(
	store *lib.InitStore,
	name string,
	entryPoint string,
	pathToBuild string,
	notify func(string),
) (*devBundleState, error) {
	state := &devBundleState{name: name, entryPoint: entryPoint}
	initialDone := false
	resultPlugin := api.Plugin{
		Name: "dev-hmr-" + name,
		Setup: func(build api.PluginBuild) {
			build.OnEnd(func(result *api.BuildResult) (api.OnEndResult, error) {
				if len(result.Errors) > 0 || len(result.OutputFiles) == 0 {
					return api.OnEndResult{}, nil
				}
				state.mu.Lock()
				state.output = append([]byte(nil), result.OutputFiles[0].Contents...)
				state.ready = true
				shouldNotify := initialDone
				initialDone = true
				state.mu.Unlock()

				if shouldNotify && notify != nil {
					notify(name)
				}
				return api.OnEndResult{}, nil
			})
		},
	}

	ctx, ctxErr := api.Context(api.BuildOptions{
		EntryPoints:   []string{entryPoint},
		AbsWorkingDir: pathToBuild,
		Bundle:        true,
		Write:         false,
		LogLevel:      api.LogLevelInfo,
		Target:        api.ES2020,
		Alias:         buildDevAliases(),
		Sourcemap:     api.SourceMapInline,
		Plugins:       []api.Plugin{resultPlugin},
	})
	if ctxErr != nil {
		return nil, ctxErr
	}

	initial := ctx.Rebuild()
	if len(initial.Errors) > 0 || len(initial.OutputFiles) == 0 {
		payload, _ := json.Marshal(initial.Errors)
		store.Logger.Warnf("Initial esbuild failed for %s: %s", name, string(payload))
	} else {
		state.mu.Lock()
		state.output = append([]byte(nil), initial.OutputFiles[0].Contents...)
		state.ready = true
		initialDone = true
		state.mu.Unlock()
	}

	if err := ctx.Watch(api.WatchOptions{Delay: 120}); err != nil {
		ctx.Dispose()
		return nil, err
	}

	state.ctx = ctx
	return state, nil
}

func startEsbuildDevHMR(store *lib.InitStore) (*esbuildDevHMR, error) {
	if !isDevEnabled(store.RetrievedSettings) {
		return nil, nil
	}

	pathToBuild := path.Join(store.RetrievedSettings.Root, "ui")
	hmr := &esbuildDevHMR{
		logger:  store.Logger,
		bundles: map[string]*devBundleState{},
		notify: func(bundleName string) {
			store.Logger.Infof("esbuild rebuild complete (%s), sending liveupdate", bundleName)
			if store.Handler != nil {
				store.Handler.BroadcastSocketEvent("liveupdate", map[string]string{"bundle": bundleName})
			}
		},
	}

	specs := []struct {
		name       string
		entryPoint string
	}{
		{name: "pad", entryPoint: "./src/pad.js"},
		{name: "welcome", entryPoint: "./src/welcome.js"},
		{name: "timeslider", entryPoint: "./src/timeslider.js"},
	}

	for _, spec := range specs {
		bundle, err := newDevBundle(store, spec.name, spec.entryPoint, pathToBuild, hmr.notify)
		if err != nil {
			return nil, err
		}
		hmr.bundles[spec.name] = bundle
	}

	store.Logger.Infof("esbuild dev watch + liveupdate enabled")
	return hmr, nil
}
