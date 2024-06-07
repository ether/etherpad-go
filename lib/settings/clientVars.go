package settings

import (
	"github.com/ether/etherpad-go/lib/utils"
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

type RootPlugin struct {
	Plugins map[string]string
	Parts   map[string]string
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
	ServerTimeStamp                    int                                `json:"serverTimestamp"`
	SessionRefreshInterval             int                                `json:"sessionRefreshInterval"`
	UserId                             string                             `json:"userId"`
	AbiwordAvailable                   bool                               `json:"abiwordAvailable"`
	SOfficeAvailable                   bool                               `json:"sofficeAvailable"`
	ExportAvailable                    bool                               `json:"exportAvailable"`
	Plugins                            RootPlugin                         `json:"plugins"`
	Parts                              map[string]interface{}             `json:"parts"`
	IndentationOnNewLine               bool                               `json:"indentationOnNewLine"`
	ScrollWhenFocusLineIsOutOfViewport ScrollWhenFocusLineIsOutOfViewport `json:"scrollWhenFocusLineIsOutOfViewport"`
	InitialChangesets                  []string                           `json:"initialChangesets"`
}

func NewClientVars() ClientVars {
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
				Text:    "test",
				Attribs: "|7+b6",
			},
			PadId:                "test",
			ClientIP:             "127.0.0.1",
			HistoricalAuthorData: historyData,
			Apool:                APool{},
			Rev:                  0,
			Time:                 0,
		},
		ColorPalette:           utils.ColorPalette,
		ClientIP:               "127.0.0.1",
		PadId:                  "test",
		UserColor:              45,
		PadOptions:             padOptions,
		PadShortcutEnabled:     padShortCutEnabled,
		InitialTitle:           "Pad: test",
		Opts:                   map[string]interface{}{},
		ChatHead:               -1,
		NumConnectedUsers:      0,
		ReadOnlyId:             "r.933623002a5d8341fbbdea37ce89f008",
		ReadOnly:               false,
		ServerTimeStamp:        1717759035617,
		SessionRefreshInterval: 86400000,
		UserId:                 "a.HrYdUXxHc5IqRn7R",
		AbiwordAvailable:       false,
		SOfficeAvailable:       false,
		ExportAvailable:        false,
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
		Plugins: RootPlugin{},
	}
}
