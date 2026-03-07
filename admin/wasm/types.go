//go:build js && wasm

package main

import (
	"sort"
	"syscall/js"
	"time"

	goapp "github.com/maxence-charriere/go-app/v10/pkg/app"
)

type updateCheckResult struct {
	CurrentVersion  string `json:"currentVersion"`
	LatestVersion   string `json:"latestVersion"`
	UpdateAvailable bool   `json:"updateAvailable"`
}

type padRecord struct {
	PadName        string `json:"padName"`
	RevisionNumber int    `json:"revisionNumber"`
	LastEdited     int64  `json:"lastEdited"`
	UserCount      int    `json:"userCount"`
}

type padsResponse struct {
	Total   int         `json:"total"`
	Results []padRecord `json:"results"`
}

type pluginRecord struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Version     string `json:"version"`
	Enabled     bool   `json:"enabled"`
}

type statsResponse struct {
	TotalUsers int `json:"totalUsers"`
}

type shoutEnvelope struct {
	Data struct {
		Payload struct {
			Timestamp int64 `json:"timestamp"`
			Message   struct {
				Message string `json:"message"`
				Sticky  bool   `json:"sticky"`
			} `json:"message"`
		} `json:"payload"`
	} `json:"data"`
}

type settingsMessage struct {
	Results any `json:"results"`
}

type installedPlugins struct {
	Installed []pluginRecord `json:"installed"`
}

type toast struct {
	Kind    string
	Message string
}

type state struct {
	CurrentPage    string
	Locale         string
	Translations   map[string]string
	Token          string
	Connected      bool
	Loading        bool
	LoadingMessage string
	Error          string
	Update         *updateCheckResult
	Pads           []padRecord
	PadsTotal      int
	PadSearch      string
	PadSort        string
	PadAscending   bool
	PadOffset      int
	PadLimit       int
	TotalUsers     int
	ShoutMessage   string
	ShoutSticky    bool
	Shouts         []shoutEnvelope
	Settings       string
	Plugins        []pluginRecord
	Toast          *toast
	LastUpdated    time.Time
}

type app struct {
	state        state
	window       js.Value
	document     js.Value
	root         js.Value
	socket       js.Value
	funcs        []js.Func
	reconnecting bool
	started      bool
	mounted      bool
	pageCtx      goapp.Context
	page         *adminPage
}

func newApp() *app {
	a := &app{
		window:   js.Global(),
		document: js.Global().Get("document"),
	}
	a.root = a.document.Call("getElementById", "root")
	a.state = state{
		CurrentPage:    "overview",
		Locale:         browserLocale(),
		Translations:   loadTranslations(),
		Token:          js.Global().Get("__adminToken").String(),
		Loading:        true,
		LoadingMessage: a.t("loading.initializing", "Initializing admin interface..."),
		PadSort:        "padName",
		PadAscending:   true,
		PadLimit:       12,
		Plugins:        []pluginRecord{},
		Pads:           []padRecord{},
		Shouts:         []shoutEnvelope{},
	}
	return a
}

func sortPluginsByName(plugins []pluginRecord) {
	sort.Slice(plugins, func(i, j int) bool {
		return plugins[i].Name < plugins[j].Name
	})
}
