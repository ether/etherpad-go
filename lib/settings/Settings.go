package settings

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"

	clientVars2 "github.com/ether/etherpad-go/lib/models/clientVars"
)

type DBSettings struct {
	Filename   string
	Host       string
	Port       string
	Database   string
	User       string
	Password   string
	Charset    string
	Collection string
	Url        string
}

type TTL struct {
	AccessToken       int
	AuthorizationCode int
	ClientCredentials int
	IdToken           int
	RefreshToken      int
}

type PadOptions struct {
	NoColors         bool
	ShowControls     bool
	ShowChat         bool
	ShowLineNumbers  bool
	UseMonospaceFont bool
	UserName         *bool
	UserColor        *bool
	RTL              bool
	AlwaysShowChat   bool
	ChatAndUsers     bool
	Lang             *string
}

type PadShortcutEnabled struct {
	AltF9     bool `json:"altF9"`
	AltC      bool `json:"altC"`
	Delete    bool `json:"delete"`
	CmdShift2 bool `json:"cmdShift2"`
	Return    bool `json:"return"`
	Esc       bool `json:"esc"`
	CmdS      bool `json:"cmdS"`
	Tab       bool `json:"tab"`
	CmdZ      bool `json:"cmdZ"`
	CmdY      bool `json:"cmdY"`
	CmdB      bool `json:"cmdB"`
	CmdI      bool `json:"cmdI"`
	CmdU      bool `json:"cmdU"`
	Cmd5      bool `json:"cmd5"`
	CmdShiftL bool `json:"cmdShiftL"`
	CmdShiftN bool `json:"cmdShiftN"`
	CmdShift1 bool `json:"cmdShift1"`
	CmdShiftC bool `json:"cmdShiftC"`
	CmdH      bool `json:"cmdH"`
	CtrlHome  bool `json:"ctrlHome"`
	PageUp    bool `json:"pageUp"`
	PageDown  bool `json:"pageDown"`
}

type Toolbar struct {
	Left       [][]string
	Right      [][]string
	TimeSlider [][]string
}

type User struct {
	Password *string `json:"password" mapstructure:"password"`
	IsAdmin  *bool   `json:"is_admin" mapstructure:"is_admin"`
	Username *string `json:"-" mapstructure:"-"`
}

type Cookie struct {
	KeyRotationInterval    int64  `json:"keyRotationInterval"`
	SameSite               string `json:"sameSite"`
	SessionLifetime        int64  `json:"sessionLifetime"`
	SessionRefreshInterval int64  `json:"sessionRefreshInterval"`
}

type SSOClient struct {
	ClientId      string   `json:"client_id" mapstructure:"client_id"`
	ClientSecret  *string  `json:"client_secret" mapstructure:"client_secret"`
	GrantTypes    []string `json:"grant_types" mapstructure:"grant_types"`
	ResponseTypes []string `json:"response_types" mapstructure:"response_types"`
	RedirectUris  []string `json:"redirect_uris" mapstructure:"redirect_uris"`
	DisplayName   string   `json:"display_name" mapstructure:"display_name"`
	Type          string   `json:"type" mapstructure:"type"`
}

type SSO struct {
	Issuer  string      `json:"issuer"`
	Clients []SSOClient `json:"clients"`
}

func (s *SSO) GetAdminClient() *SSOClient {
	for _, client := range s.Clients {
		if client.Type == "admin" {
			return &client
		}
	}
	return nil
}

type Cleanup struct {
	Enabled       bool `json:"enabled"`
	KeepRevisions int  `json:"keepRevisions"`
}

type ImportExportRateLimiting struct {
	WindowMS int `json:"windowMS"`
	Max      int `json:"max"`
}

type CommitRateLimiting struct {
	Duration int `json:"duration"`
	Points   int `json:"points"`
}

type SSLSettings struct {
	Key  string   `json:"key"`
	Cert string   `json:"cert"`
	Ca   []string `json:"ca"`
}

type Settings struct {
	Root                               string
	SettingsFilename                   string                                         `json:"settingsFilename"`
	CredentialsFilename                string                                         `json:"credentialsFilename"`
	Title                              string                                         `json:"title"`
	ShowRecentPads                     bool                                           `json:"showRecentPads"`
	Favicon                            *string                                        `json:"favicon"`
	TTL                                TTL                                            `json:"ttl"`
	UpdateServer                       string                                         `json:"updateServer"`
	EnableDarkMode                     bool                                           `json:"enableDarkMode"`
	SkinName                           string                                         `json:"skinName"`
	SkinVariants                       string                                         `json:"skinVariants"`
	IP                                 string                                         `json:"ip"`
	Port                               string                                         `json:"port"`
	SuppressErrorsInPadText            bool                                           `json:"suppressErrorsInPadText"`
	SSL                                SSLSettings                                    `json:"ssl"`
	DBType                             IDBType                                        `json:"dbType"`
	DBSettings                         *DBSettings                                    `json:"dbSettings"`
	DefaultPadText                     string                                         `json:"defaultPadText"`
	PadOptions                         PadOptions                                     `json:"padOptions"`
	EnableMetrics                      bool                                           `json:"enableMetrics"`
	PadShortCutEnabled                 PadShortcutEnabled                             `json:"padShortCutEnabled"`
	Toolbar                            Toolbar                                        `json:"toolbar"`
	RequireSession                     bool                                           `json:"requireSession"`
	EditOnly                           bool                                           `json:"editOnly"`
	MaxAge                             int                                            `json:"maxAge"`
	Minify                             bool                                           `json:"minify"`
	Abiword                            *string                                        `json:"abiword"`
	SOffice                            *string                                        `json:"soffice"`
	AllowUnknownFileEnds               bool                                           `json:"allowUnknownFileEnds"`
	LogLevel                           string                                         `json:"logLevel"`
	DisableIPLogging                   bool                                           `json:"disableIPLogging"`
	AutomaticReconnectionTimeout       int                                            `json:"automaticReconnectionTimeout"`
	LoadTest                           bool                                           `json:"loadTest"`
	DumpOnCleanExit                    bool                                           `json:"dumpOnCleanExit"`
	IndentationOnNewLine               bool                                           `json:"indentationOnNewLine"`
	SessionKey                         *string                                        `json:"SessionKey"`
	TrustProxy                         bool                                           `json:"trustProxy"`
	Cookie                             Cookie                                         `json:"cookie"`
	RequireAuthentication              bool                                           `json:"requireAuthentication"`
	RequireAuthorization               bool                                           `json:"requireAuthorization"`
	Users                              map[string]User                                `json:"users"`
	ShowSettingsInAdminPage            bool                                           `json:"showSettingsInAdminPage"`
	ScrollWhenFocusLineIsOutOfViewport clientVars2.ScrollWhenFocusLineIsOutOfViewport `json:"scrollWhenFocusLineIsOutOfViewport"`
	SocketIo                           SocketIoSettings                               `json:"socketIo"`
	AuthenticationMethod               string                                         `json:"authenticationMethod"`
	SSO                                *SSO                                           `json:"sso"`
	Cleanup                            Cleanup                                        `json:"cleanup"`
	ExposeVersion                      bool                                           `json:"exposeVersion"`
	CustomLocaleStrings                map[string]map[string]string                   `json:"customLocaleStrings"`
	ImportExportRateLimiting           ImportExportRateLimiting                       `json:"importExportRateLimiting"`
	CommitRateLimiting                 CommitRateLimiting                             `json:"commitRateLimiting"`
	ImportMaxFileSize                  int64                                          `json:"importMaxFileSize"`
	EnableAdminUITests                 bool                                           `json:"enableAdminUITests"`
	LowerCasePadIDs                    bool                                           `json:"lowerCasePadIds"`
	RandomVersionString                string                                         `json:"randomVersionString"`
	DevMode                            bool                                           `json:"devMode"`
	GitVersion                         string                                         `json:"-"`
	AvailableExports                   []string                                       `json:"availableExports"`
}

func (s *Settings) GetPublicSettings() PublicSettings {
	return PublicSettings{
		GitVersion:          s.GitVersion,
		Toolbar:             s.Toolbar,
		ExposeVersion:       s.ExposeVersion,
		RandomVersionString: s.RandomVersionString,
		Title:               s.Title,
		SkinName:            s.SkinName,
		SkinVariants:        s.SkinVariants,
	}
}

func (s *Settings) abiwordAvailable() string {
	if s.Abiword != nil {
		if runtime.GOOS == "windows" {
			return "withoutPDF"
		}
		return "yes"
	}
	return "no"
}

func (s *Settings) sofficeAvailable() string {
	if s.SOffice != nil {
		if runtime.GOOS == "windows" {
			return "withoutPDF"
		}
		return "yes"
	}
	return "no"
}

func (s *Settings) ExportToExternalToolsAvailable() string {
	var abiword = s.abiwordAvailable()
	var soffice = s.sofficeAvailable()

	if abiword == "no" && soffice == "no" {
		return "no"
	}
	return "yes"
}

type SocketIoSettings struct {
	MaxHttpBufferSize int64 `json:"maxHttpBufferSize"`
}

var Displayed Settings

func stripWithoutWhitespace() string {
	return ""
}

var rgx = regexp.MustCompile(`\S`)

func stripWithWhitespace(string string, start *int, end *int) string {
	// slice only if start and end are not nil
	if start != nil && end != nil {
		string = string[*start:*end]
	} else if start != nil {
		string = string[*start:]
	}

	return rgx.ReplaceAllString(string, " ")
}

func isEscaped(jsonString string, quotePosition int) bool {
	index := quotePosition - 1
	backslashCount := 0

	for string(jsonString[index]) == "\\" {
		index -= 1
		backslashCount += 1
	}

	return backslashCount%2 == 1
}

type Options struct {
	Whitespace     bool
	TrailingCommas bool
}

const notInsideComment = 0
const singleComment = 1
const multiComment = 2

func StripWithOptions(jsonString string, options *Options) string {

	// if options are not provided, use default options
	// whitespace: true
	// trailingCommas: false
	if options == nil {
		options = &Options{Whitespace: true, TrailingCommas: false}
	}

	isInsideString := false
	isInsideComment := notInsideComment
	offset := 0
	buffer := ""
	result := ""
	commaIndex := -1

	// shorthand function
	strip := func(index int) string {
		if options.Whitespace {
			return stripWithWhitespace(jsonString, &offset, &index)
		} else {
			return stripWithoutWhitespace()
		}
	}

	for index := 0; index < len(jsonString); index++ {
		currentCharacter := string(jsonString[index])
		nextCharacter := ""

		if index+1 < len(jsonString) {
			nextCharacter = string(jsonString[index+1])
		}

		if isInsideComment == notInsideComment && currentCharacter == `"` {
			// Enter or exit string
			escaped := isEscaped(jsonString, index)
			if !escaped {
				isInsideString = !isInsideString
			}
		}

		if isInsideString {
			continue
		}

		if isInsideComment == notInsideComment && currentCharacter+nextCharacter == "//" {
			// Enter single-line comment
			buffer += jsonString[offset:index]
			offset = index
			isInsideComment = singleComment
			index++
		} else if isInsideComment == singleComment && currentCharacter+nextCharacter == "\r\n" {
			// Exit single-line comment via \r\n
			index++
			isInsideComment = notInsideComment
			buffer += strip(index)
			offset = index
		} else if isInsideComment == singleComment && currentCharacter == "\n" {
			// Exit single-line comment via \n
			isInsideComment = notInsideComment
			buffer += strip(index)
			offset = index
		} else if isInsideComment == notInsideComment && currentCharacter+nextCharacter == "/*" {
			// Enter multiline comment
			buffer += jsonString[offset:index]
			offset = index
			isInsideComment = multiComment
			index++

		} else if isInsideComment == multiComment && currentCharacter+nextCharacter == "*/" {
			// Exit multiline comment
			index++
			isInsideComment = notInsideComment
			buffer += strip(index + 1)
			offset = index + 1

		} else if options.TrailingCommas && isInsideComment == notInsideComment {
			if commaIndex != -1 {
				if currentCharacter == "}" || currentCharacter == "]" {
					// Strip trailing comma
					buffer += jsonString[offset:index]
					if options.Whitespace {
						s, e := 0, 1
						result += stripWithWhitespace(jsonString, &s, &e)
					} else {
						result += stripWithoutWhitespace()
					}
					result += buffer[1:]
					buffer = ""
					offset = index
					commaIndex = -1
				} else if currentCharacter != " " && currentCharacter != "\t" && currentCharacter != "\r" && currentCharacter != "\n" {
					// Hit non-whitespace following a comma; comma is not trailing
					buffer += jsonString[offset:index]
					offset = index
					commaIndex = -1
				}
			} else if currentCharacter == "," {
				// Flush buffer prior to this point, and save new comma index
				result += buffer + jsonString[offset:index]
				buffer = ""
				offset = index
				commaIndex = index

			}
		}
	}

	var end string
	if isInsideComment > notInsideComment {
		if options.Whitespace {
			end = stripWithWhitespace(jsonString[offset:], nil, nil)
		} else {
			end = stripWithoutWhitespace()
		}

	} else {
		end = jsonString[offset:]
	}

	return result + buffer + end
}

func init() {
	var pathToRoot string

	var envPathToSettings = os.Getenv("ETHERPAD_SETTINGS_PATH")
	if envPathToSettings != "" {
		pathToRoot = envPathToSettings
	}

	if pathToRoot == "" {
		for i := 0; i < 10; i++ {
			var builtPath = ""
			for j := 0; j < i; j++ {
				builtPath += "../"
			}

			var assetDir string

			if i == 0 {
				assetDir = "./assets"
			} else {
				assetDir = "assets"
			}

			pathToAssets, err := filepath.Abs(builtPath + assetDir)

			_, err = os.Stat(pathToAssets)

			if err == nil {
				pathToRoot, _ = filepath.Abs(builtPath)
				break
			}

			if i == 9 {
				panic("Error finding root")
			}
		}
	}

	var settingsFilePath = filepath.Join(pathToRoot, "settings.json")
	settings, err := os.ReadFile(settingsFilePath)
	settings = []byte(StripWithOptions(string(settings), &Options{Whitespace: true, TrailingCommas: true}))

	setting, err := ReadConfig(string(settings))
	if err != nil {
		println("error is " + err.Error())
		return
	}
	setting.GitVersion = GitVersion()
	setting.Root = pathToRoot
	Displayed = *setting

}
