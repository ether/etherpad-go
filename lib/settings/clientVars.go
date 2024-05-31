package settings

import "github.com/ether/etherpad-go/lib/apool"

type AccountPrivs struct {
	MaxRevisions int `json:"maxRevisions"`
}

type CollabClientVars struct {
	InitialAttributedText apool.AText `json:"initialAttributedText"`
	ClientIP              string      `json:"clientIp"`
	PadId                 string      `json:"padId"`
	Rev                   int         `json:"rev"`
	Time                  int         `json:"time"`
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

type ClientVars struct {
	SkinName                           string                                       `json:"skinName"`
	SkinVariants                       string                                       `json:"skinVariants"`
	RandomVersionString                string                                       `json:"randomVersionString"`
	AccountPrivs                       AccountPrivs                                 `json:"accountPrivs"`
	AutomaticReconnectionTimeout       int                                          `json:"automaticReconnectionTimeout"`
	InitialRevisionList                []string                                     `json:"initialRevisionList"`
	InitialOptions                     map[string]interface{}                       `json:"initialOptions"`
	SavedRevisions                     []string                                     `json:"savedRevisions"`
	CollabClientVars                   CollabClientVars                             `json:"collab_client_vars"`
	ColorPalette                       []string                                     `json:"colorPalette"`
	ClientIP                           string                                       `json:"clientIp"`
	UserColor                          string                                       `json:"userColor"`
	PadId                              string                                       `json:"padId"`
	PadOptions                         map[string]interface{}                       `json:"padOptions"`
	PadShortcutEnabled                 map[string]bool                              `json:"padShortcutEnabled"`
	InitialTitle                       string                                       `json:"initialTitle"`
	Opts                               map[string]interface{}                       `json:"opts"`
	ChatHead                           int                                          `json:"chatHead"`
	NumConnectedUsers                  int                                          `json:"numConnectedUsers"`
	ReadOnlyId                         string                                       `json:"readOnlyId"`
	ReadOnly                           bool                                         `json:"readOnly"`
	ServerTimeStamp                    int                                          `json:"serverTimestamp"`
	SessionRefreshInterval             int                                          `json:"sessionRefreshInterval"`
	UserId                             string                                       `json:"userId"`
	AbiwordAvailable                   bool                                         `json:"abiwordAvailable"`
	SOfficeAvailable                   bool                                         `json:"sofficeAvailable"`
	ExportAvailable                    bool                                         `json:"exportAvailable"`
	Plugins                            map[string]interface{}                       `json:"plugins"`
	Parts                              map[string]interface{}                       `json:"parts"`
	IndentationOnNewLine               bool                                         `json:"indentationOnNewLine"`
	ScrollWhenFocusLineIsOutOfViewport ScrollWhenFocusLineIsOutOfViewportPercentage `json:"scrollWhenFocusLineIsOutOfViewport"`
	InitialChangesets                  []string                                     `json:"initialChangesets"`
}
