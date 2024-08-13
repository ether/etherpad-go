package clientVars

import (
	apool2 "github.com/ether/etherpad-go/lib/apool"
	author2 "github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/models/ws"
	pad2 "github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/utils"
	"time"
)

func NewClientVars(pad pad.Pad, sessionInfo *ws.Session, apool apool2.APool) clientVars.ClientVars {
	var historyData = make(map[string]clientVars.CollabAuthor)
	var readonlyManager = pad2.NewReadOnlyManager()
	var allauthors = pad.GetAllAuthors()

	for _, author := range allauthors {
		manager := author2.NewManager()
		var retrievedAuthor, err = manager.GetAuthor(author)

		if err != nil {
			continue
		}

		historyData[author] = clientVars.CollabAuthor{
			Name:    retrievedAuthor.Name,
			ColorId: &utils.ColorPalette[retrievedAuthor.ColorId],
		}
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

	var rootPlugin = clientVars.RootPlugin{
		Plugins: make(map[string]clientVars.PluginInMessage),
		Parts:   make(map[string]clientVars.PartInMessage),
	}

	var plugins = utils.GetPlugins()
	for s := range plugins {
		var rawParts = utils.GetParts()
		var convertedParts = make([]clientVars.PartInMessage, 0)
		for part := range rawParts {
			if rawParts[part].Plugin != nil && *rawParts[part].Plugin == s {
				convertedParts = append(convertedParts, clientVars.PartInMessage{
					Name:     rawParts[part].Name,
					Plugin:   *rawParts[part].Plugin,
					Hooks:    rawParts[part].Hooks,
					FullName: *rawParts[part].FullName,
				})
			}
		}
		rootPlugin.Plugins[s] = clientVars.PluginInMessage{
			Parts: convertedParts,
			Package: clientVars.PluginInMessagePackage{
				Name:     plugins[s].Name,
				Path:     plugins[s].Path,
				RealPath: plugins[s].RealPath,
				Version:  plugins[s].Version,
			},
		}
	}

	var currentTime = pad.GetRevisionDate(pad.Head)
	var readonlyId = readonlyManager.GetIds(&pad.Id)

	return clientVars.ClientVars{
		SkinName:            "colibris",
		SkinVariants:        "super-light-toolbar super-light-editor light-background",
		RandomVersionString: "f2cb49c4",
		AccountPrivs: clientVars.AccountPrivs{
			MaxRevisions: 100,
		},
		AutomaticReconnectionTimeout: 0,
		InitialRevisionList:          make([]string, 0),
		InitialOptions:               make(map[string]interface{}),
		SavedRevisions:               make([]string, 0),
		CollabClientVars: clientVars.CollabClientVars{
			InitialAttributedText: clientVars.InitialAttributedText{
				Text:    pad.AText.Text,
				Attribs: pad.AText.Attribs,
			},
			PadId:                pad.Id,
			ClientIP:             "127.0.0.1",
			HistoricalAuthorData: historyData,
			Apool: clientVars.APool{
				NumToAttrib: apool.NumToAttrib,
				NextNum:     apool.NextNum,
			},
			Rev:  pad.Head,
			Time: currentTime,
		},
		ColorPalette:           utils.ColorPalette,
		ClientIP:               "127.0.0.1",
		PadId:                  pad.Id,
		UserColor:              utils.ColorPalette[1],
		PadOptions:             padOptions,
		PadShortcutEnabled:     padShortCutEnabled,
		InitialTitle:           "Pad: " + pad.Id,
		Opts:                   map[string]interface{}{},
		ChatHead:               -1,
		NumConnectedUsers:      0,
		ReadOnlyId:             readonlyId.ReadOnlyPadId,
		ReadOnly:               readonlyId.ReadOnly,
		ServerTimeStamp:        int64(time.Now().Second()),
		SessionRefreshInterval: 86400000,
		UserId:                 sessionInfo.Author,
		AbiwordAvailable:       "no",
		SOfficeAvailable:       "no",
		ExportAvailable:        "no",
		IndentationOnNewLine:   true,
		ScrollWhenFocusLineIsOutOfViewport: clientVars.ScrollWhenFocusLineIsOutOfViewport{
			Percentage: clientVars.ScrollWhenFocusLineIsOutOfViewportPercentage{
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
