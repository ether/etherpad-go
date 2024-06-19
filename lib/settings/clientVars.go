package settings

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/utils"
	"time"
)

type AccountPrivs struct {
	MaxRevisions int `json:"maxRevisions"`
}

type InitialAttributedText struct {
	Text    string `json:"text"`
	Attribs string `json:"attribs"`
}

type CollabAuthor struct {
	Name    string `json:"name"`
	ColorId int    `json:"colorId"`
}

type APool struct {
	NumToAttrib map[int]apool.Attribute `json:"numToAttrib"`
	NextNum     int                     `json:"nextNum"`
}

type CollabClientVars struct {
	InitialAttributedText InitialAttributedText   `json:"initialAttributedText"`
	ClientIP              string                  `json:"clientIp"`
	PadId                 string                  `json:"padId"`
	HistoricalAuthorData  map[string]CollabAuthor `json:"historicalAuthorData"`
	Apool                 APool                   `json:"apool"`
	Rev                   int                     `json:"rev"`
	Time                  int                     `json:"time"`
}

type ScrollWhenFocusLineIsOutOfViewportPercentage struct {
	EditionAboveViewport int `json:"editionAboveViewport"`
	EditionBelowViewport int `json:"editionBelowViewport"`
}

type ScrollWhenFocusLineIsOutOfViewport struct {
	Percentage                               ScrollWhenFocusLineIsOutOfViewportPercentage `json:"percentage"`
	Duration                                 int                                          `json:"duration"`
	ScrollWhenCaretIsInTheLastLineOfViewport bool                                         `json:"scrollWhenCaretIsInTheLastLineOfViewport"`
	PercentageToScrollWhenUserPressesArrowUp int                                          `json:"percentageToScrollWhenUserPressesArrowUp"`
}

type PluginInMessagePackage struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	RealPath string `json:"realPath"`
	Version  string `json:"version"`
}

type PartInMessage struct {
	FullName string            `json:"full_name"`
	Hooks    map[string]string `json:"hooks"`
	Name     string            `json:"name"`
	Plugin   string            `json:"plugin"`
}

type PluginInMessage struct {
	Package PluginInMessagePackage `json:"package"`
	Parts   []PartInMessage        `json:"parts"`
}

type RootPlugin struct {
	Plugins map[string]PluginInMessage `json:"plugins"`
	Parts   map[string]PartInMessage   `json:"parts"`
}

type ClientVars struct {
	SkinName                           string                             `json:"skinName"`
	SkinVariants                       string                             `json:"skinVariants"`
	RandomVersionString                string                             `json:"randomVersionString"`
	AccountPrivs                       AccountPrivs                       `json:"accountPrivs"`
	AutomaticReconnectionTimeout       int                                `json:"automaticReconnectionTimeout"`
	InitialRevisionList                []string                           `json:"initialRevisionList"`
	InitialOptions                     map[string]interface{}             `json:"initialOptions"`
	SavedRevisions                     []string                           `json:"savedRevisions"`
	CollabClientVars                   CollabClientVars                   `json:"collab_client_vars"`
	ColorPalette                       []string                           `json:"colorPalette"`
	ClientIP                           string                             `json:"clientIp"`
	UserColor                          int                                `json:"userColor"`
	PadId                              string                             `json:"padId"`
	PadOptions                         map[string]bool                    `json:"padOptions"`
	PadShortcutEnabled                 map[string]bool                    `json:"padShortcutEnabled"`
	InitialTitle                       string                             `json:"initialTitle"`
	Opts                               map[string]interface{}             `json:"opts"`
	ChatHead                           int                                `json:"chatHead"`
	NumConnectedUsers                  int                                `json:"numConnectedUsers"`
	ReadOnlyId                         string                             `json:"readOnlyId"`
	ReadOnly                           bool                               `json:"readOnly"`
	ServerTimeStamp                    int64                              `json:"serverTimestamp"`
	SessionRefreshInterval             int                                `json:"sessionRefreshInterval"`
	UserId                             string                             `json:"userId"`
	AbiwordAvailable                   string                             `json:"abiwordAvailable"`
	SOfficeAvailable                   string                             `json:"sofficeAvailable"`
	ExportAvailable                    string                             `json:"exportAvailable"`
	Plugins                            RootPlugin                         `json:"plugins"`
	Parts                              map[string]interface{}             `json:"parts"`
	IndentationOnNewLine               bool                               `json:"indentationOnNewLine"`
	ScrollWhenFocusLineIsOutOfViewport ScrollWhenFocusLineIsOutOfViewport `json:"scrollWhenFocusLineIsOutOfViewport"`
	InitialChangesets                  []string                           `json:"initialChangesets"`
}

func NewClientVars(pad pad.Pad) ClientVars {
	var historyData = make(map[string]CollabAuthor)
	historyData["a.HrYdUXxHc5IqRn7R"] = CollabAuthor{
		Name:    "test",
		ColorId: 45,
	}

	var padOptions = make(map[string]bool)

	padOptions["noColors"] = false
	padOptions["showControl"] = true
	padOptions["showChat"] = true
	padOptions["showLineNumbers"] = true
	padOptions["useMonospaceFont"] = false
	padOptions["userName"] = false
	padOptions["userColor"] = false
	padOptions["rtl"] = false
	padOptions["alwaysShowChat"] = false
	padOptions["chatAndUsers"] = false
	padOptions["lang"] = false

	var padShortCutEnabled = make(map[string]bool)
	padShortCutEnabled["altF9"] = true
	padShortCutEnabled["altC"] = true
	padShortCutEnabled["delete"] = true
	padShortCutEnabled["cmdShift2"] = true
	padShortCutEnabled["return"] = true
	padShortCutEnabled["esc"] = true
	padShortCutEnabled["cmdS"] = true
	padShortCutEnabled["tab"] = true
	padShortCutEnabled["cmdZ"] = true
	padShortCutEnabled["cmdY"] = true
	padShortCutEnabled["cmdB"] = true
	padShortCutEnabled["cmdI"] = true
	padShortCutEnabled["cmdU"] = true
	padShortCutEnabled["cmd5"] = true
	padShortCutEnabled["cmdShiftL"] = true
	padShortCutEnabled["cmdShiftN"] = true
	padShortCutEnabled["cmdShift1"] = true
	padShortCutEnabled["cmdShiftC"] = true
	padShortCutEnabled["cmdH"] = true
	padShortCutEnabled["ctrlHome"] = true
	padShortCutEnabled["pageUp"] = true
	padShortCutEnabled["pageDown"] = true

	var rootPlugin = RootPlugin{
		Plugins: make(map[string]PluginInMessage),
		Parts:   make(map[string]PartInMessage),
	}

	var plugins = utils.GetPlugins()
	for s := range plugins {
		var rawParts = utils.GetParts()
		var convertedParts = make([]PartInMessage, 0)
		for part := range rawParts {
			if rawParts[part].Plugin != nil && *rawParts[part].Plugin == s {
				convertedParts = append(convertedParts, PartInMessage{
					Name:     rawParts[part].Name,
					Plugin:   *rawParts[part].Plugin,
					Hooks:    rawParts[part].Hooks,
					FullName: *rawParts[part].FullName,
				})
			}
		}
		rootPlugin.Plugins[s] = PluginInMessage{
			Parts: convertedParts,
			Package: PluginInMessagePackage{
				Name:     plugins[s].Name,
				Path:     plugins[s].Path,
				RealPath: plugins[s].RealPath,
				Version:  plugins[s].Version,
			},
		}
	}

	return ClientVars{
		SkinName:            "colibris",
		SkinVariants:        "super-light-toolbar super-light-editor light-background",
		RandomVersionString: "f2cb49c4",
		AccountPrivs: AccountPrivs{
			MaxRevisions: 100,
		},
		AutomaticReconnectionTimeout: 0,
		InitialRevisionList:          make([]string, 0),
		InitialOptions:               make(map[string]interface{}),
		SavedRevisions:               make([]string, 0),
		CollabClientVars: CollabClientVars{
			InitialAttributedText: InitialAttributedText{
				Text:    pad.AText.Text,
				Attribs: pad.AText.Attribs,
			},
			PadId:                pad.Id,
			ClientIP:             "127.0.0.1",
			HistoricalAuthorData: historyData,
			Apool: APool{
				NumToAttrib: make(map[int]apool.Attribute),
				NextNum:     0,
			},
			Rev:  0,
			Time: 0,
		},
		ColorPalette:           utils.ColorPalette,
		ClientIP:               "127.0.0.1",
		PadId:                  pad.Id,
		UserColor:              1,
		PadOptions:             padOptions,
		PadShortcutEnabled:     padShortCutEnabled,
		InitialTitle:           "Pad: " + pad.Id,
		Opts:                   map[string]interface{}{},
		ChatHead:               -1,
		NumConnectedUsers:      0,
		ReadOnlyId:             "r.933623002a5d8341fbbdea37ce89f008",
		ReadOnly:               false,
		ServerTimeStamp:        int64(time.Now().Second()),
		SessionRefreshInterval: 86400000,
		UserId:                 "a.HrYdUXxHc5IqRn7R",
		AbiwordAvailable:       "no",
		SOfficeAvailable:       "no",
		ExportAvailable:        "no",
		IndentationOnNewLine:   true,
		ScrollWhenFocusLineIsOutOfViewport: ScrollWhenFocusLineIsOutOfViewport{
			Percentage: ScrollWhenFocusLineIsOutOfViewportPercentage{
				EditionAboveViewport: 0,
				EditionBelowViewport: 0,
			},
			Duration:                                 0,
			ScrollWhenCaretIsInTheLastLineOfViewport: false,
			PercentageToScrollWhenUserPressesArrowUp: 0,
		},
		Plugins: rootPlugin,
	}
}
