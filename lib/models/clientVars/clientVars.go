package clientVars

import "github.com/ether/etherpad-go/lib/apool"

type AccountPrivs struct {
	MaxRevisions int `json:"maxRevisions"`
}

type InitialAttributedText struct {
	Text    string `json:"text"`
	Attribs string `json:"attribs"`
}

type CollabAuthor struct {
	Name    *string `json:"name"`
	ColorId *string `json:"colorId"`
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
	UserColor                          string                             `json:"userColor"`
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
