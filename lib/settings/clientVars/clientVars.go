package clientVars

import (
	"strconv"
	"time"

	apool2 "github.com/ether/etherpad-go/lib/apool"
	author2 "github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/models/ws"
	pad2 "github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
)

type Factory struct {
	ReadOnlyManager *pad2.ReadOnlyManager
	AuthorManager   *author2.Manager
}

func (f *Factory) NewClientVars(pad pad.Pad, sessionInfo *ws.Session, apool apool2.APool, translatedAttribs string, historicalAuthorData map[string]author2.Author, retrievedSettings *settings.Settings) (*clientVars.ClientVars, error) {
	var historyData = make(map[string]clientVars.CollabAuthor)

	for _, authorData := range historicalAuthorData {
		historyData[authorData.Id] = clientVars.CollabAuthor{
			Name:    authorData.Name,
			ColorId: authorData.ColorId,
		}
	}

	var currentAuthor, err = f.AuthorManager.GetAuthor(sessionInfo.Author)
	if err != nil {
		return nil, err
	}

	var padOptions = make(map[string]*bool)

	var boolTrue = true
	var boolFalse = false

	padOptions["noColors"] = &boolFalse
	padOptions["showControls"] = &boolTrue
	padOptions["showChat"] = &boolTrue
	padOptions["showLineNumbers"] = &boolTrue
	padOptions["useMonospaceFont"] = &boolFalse
	padOptions["userName"] = nil
	padOptions["userColor"] = nil
	padOptions["rtl"] = &boolFalse
	padOptions["alwaysShowChat"] = &boolFalse
	padOptions["chatAndUsers"] = &boolFalse
	padOptions["lang"] = nil

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

	currentTime, err := pad.GetRevisionDate(pad.Head)
	if err != nil {
		return nil, err
	}

	etherPadConvertedAttribs := make(map[string][]string)
	for k, v := range apool.NumToAttrib {
		etherPadConvertedAttribs[strconv.Itoa(k)] = v.ToStringSlice()
	}

	var abiwordAvailable = "no"
	if retrievedSettings.Abiword != nil && *retrievedSettings.Abiword != "" {
		abiwordAvailable = "yes"
	}

	var sofficeAvailable = "no"
	if retrievedSettings.SOffice != nil && *retrievedSettings.SOffice != "" {
		sofficeAvailable = "yes"
	}

	return &clientVars.ClientVars{
		SkinName:            retrievedSettings.SkinName,
		SkinVariants:        retrievedSettings.SkinVariants,
		RandomVersionString: "f2cb49c4",
		AccountPrivs: clientVars.AccountPrivs{
			MaxRevisions: 100,
		},
		AutomaticReconnectionTimeout: retrievedSettings.AutomaticReconnectionTimeout,
		InitialRevisionList:          make([]string, 0),
		InitialOptions:               make(map[string]interface{}),
		SavedRevisions:               make([]string, 0),
		CollabClientVars: clientVars.CollabClientVars{
			InitialAttributedText: clientVars.InitialAttributedText{
				Text:    pad.AText.Text,
				Attribs: translatedAttribs,
			},
			PadId:                pad.Id,
			ClientIP:             "127.0.0.1",
			HistoricalAuthorData: historyData,
			Apool: clientVars.APool{
				NumToAttrib: etherPadConvertedAttribs,
				NextNum:     apool.NextNum,
			},
			Rev:  pad.Head,
			Time: *currentTime,
		},
		ColorPalette:                       utils.ColorPalette,
		ClientIP:                           "127.0.0.1",
		PadId:                              pad.Id,
		UserColor:                          currentAuthor.ColorId,
		PadOptions:                         padOptions,
		PadShortcutEnabled:                 padShortCutEnabled,
		InitialTitle:                       "Pad: " + pad.Id,
		Opts:                               map[string]interface{}{},
		ChatHead:                           pad.ChatHead,
		NumConnectedUsers:                  0,
		ReadOnlyId:                         sessionInfo.ReadOnlyPadId,
		ReadOnly:                           sessionInfo.ReadOnly,
		ServerTimeStamp:                    time.Now().UTC().UnixMilli(),
		SessionRefreshInterval:             86400000,
		UserName:                           currentAuthor.Name,
		UserId:                             sessionInfo.Author,
		AbiwordAvailable:                   abiwordAvailable,
		SOfficeAvailable:                   sofficeAvailable,
		AvailableExports:                   retrievedSettings.AvailableExports,
		IndentationOnNewLine:               retrievedSettings.IndentationOnNewLine,
		ScrollWhenFocusLineIsOutOfViewport: retrievedSettings.ScrollWhenFocusLineIsOutOfViewport,
		Plugins:                            rootPlugin,
		InitialChangesets:                  make([]string, 0),
	}, nil
}
