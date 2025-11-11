package settings

import (
	"os"
	"path"
	"path/filepath"
	"reflect"
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
	Password *string `json:"password"`
	IsAdmin  *bool   `json:"is_admin"`
	Username *string `json:"-"`
}

type Cookie struct {
	KeyRotationInterval    int64  `json:"keyRotationInterval"`
	SameSite               string `json:"sameSite"`
	SessionLifetime        int64  `json:"sessionLifetime"`
	SessionRefreshInterval int64  `json:"sessionRefreshInterval"`
}

type SSOClients struct {
	ClientId      string   `json:"client_id"`
	ClientSecret  string   `json:"client_secret"`
	GrantTypes    []string `json:"grant_types"`
	ResponseTypes []string `json:"response_types"`
	RedirectUris  []string `json:"redirect_uris"`
}

type SSO struct {
	Issuer  string       `json:"issuer"`
	Clients []SSOClients `json:"clients"`
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
	DBType                             string                                         `json:"dbType"`
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
	GitVersion                         string                                         `json:"-"`
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

func (s *Settings) ExportAvailable() string {
	var abiword = s.abiwordAvailable()
	var soffice = s.sofficeAvailable()

	if abiword == "no" && soffice == "no" {
		return "no"
	} else if (abiword == "withoutPDF" && soffice == "no") || (soffice == "withoutPDF" && abiword == "no") {
		return "withoutPDF"
	}
	return "yes"
}

type SocketIoSettings struct {
	MaxHttpBufferSize int64 `json:"maxHttpBufferSize"`
}

var Displayed Settings

func newDefaultSettings(pathToRoot string) Settings {
	return Settings{
		Root:                pathToRoot,
		SettingsFilename:    path.Join(pathToRoot, "settings.json"),
		CredentialsFilename: path.Join(pathToRoot, "credentials.json"),
		/**
		 * The app title, visible e.g. in the browser window
		 */
		Title: "Etherpad",
		/**
		 * Whether to show recent pads on the homepage
		 */
		ShowRecentPads: true,
		Favicon:        nil,
		TTL: TTL{
			AccessToken:       1 * 60 * 60,      // 1 hour in seconds
			AuthorizationCode: 10 * 60,          // 10 minutes in seconds
			ClientCredentials: 1 * 60 * 60,      // 1 hour in seconds
			IdToken:           1 * 60 * 60,      // 1 hour in seconds
			RefreshToken:      1 * 24 * 60 * 60, // 1 day in seconds
		},

		UpdateServer:   "https://static.etherpad.org",
		EnableDarkMode: true,
		/*
		 * Skin name.
		 *
		 * Initialized to null, so we can spot an old configuration file and invite the
		 * user to update it before falling back to the default.
		 */
		SkinName:                "colibris",
		SkinVariants:            "super-light-toolbar super-light-editor light-background",
		IP:                      "0.0.0.0",
		Port:                    "9001",
		SuppressErrorsInPadText: false,
		SocketIo: SocketIoSettings{
			/**
			 * Maximum permitted client message size (in bytes).
			 *
			 * All messages from clients that are larger than this will be rejected. Large values make it
			 * possible to paste large amounts of text, and plugins may require a larger value to work
			 * properly, but increasing the value increases susceptibility to denial of service attacks
			 * (malicious clients can exhaust memory).
			 */
			MaxHttpBufferSize: 50000,
		},
		AuthenticationMethod: "sso",
		DBType:               "rustydb",
		DBSettings:           nil,
		DefaultPadText: `Welcome to Etherpad!

This pad text is synchronized as you type, so that everyone viewing this page sees the same text. This allows you to collaborate seamlessly on documents!

Etherpad on Github: https://github.com/ether/etherpad-lite`,
		/**
		 * The default Pad Settings for a user (Can be overridden by changing the setting
		 */
		PadOptions: PadOptions{
			NoColors:         false,
			ShowControls:     true,
			ShowChat:         true,
			ShowLineNumbers:  true,
			UseMonospaceFont: false,
			UserName:         nil,
			UserColor:        nil,
			RTL:              false,
			AlwaysShowChat:   false,
			ChatAndUsers:     false,
			Lang:             nil,
		},
		/**
		 * Whether to enable the /stats endpoint. The functionality in the admin menu is untouched for this.
		 */
		EnableMetrics: true,
		PadShortCutEnabled: PadShortcutEnabled{
			AltF9:     true,
			AltC:      true,
			Delete:    true,
			CmdShift2: true,
			Return:    true,
			Esc:       true,
			CmdS:      true,
			Tab:       true,
			CmdZ:      true,
			CmdY:      true,
			CmdB:      true,
			CmdI:      true,
			CmdU:      true,
			Cmd5:      true,
			CmdShiftL: true,
			CmdShiftN: true,
			CmdShift1: true,
			CmdShiftC: true,
			CmdH:      true,
			CtrlHome:  true,
			PageUp:    true,
			PageDown:  true,
		},
		/**
		 * The toolbar buttons and order.
		 */
		Toolbar: Toolbar{
			Left: [][]string{
				{"bold", "italic", "underline", "strikethrough"},
				{"orderedlist", "unorderedlist", "indent", "outdent"},
				{"undo", "redo"},
				{"clearauthorship"},
			},
			Right: [][]string{
				{"importexport", "timeslider", "savedrevision"},
				{"settings", "embed", "showusers"},
			},
			TimeSlider: [][]string{
				{"timeslider_export", "timeslider_settings", "timeslider_returnToPad"},
			},
		},
		/**
		 * A flag that requires any user to have a valid session (via the api) before accessing a pad
		 */
		RequireSession: false,
		/**
		 * A flag that prevents users from creating new pads
		 */
		EditOnly: false,
		/**
		 * Max age that responses will have (affects caching layer).
		 */
		MaxAge: 1000 * 60 * 60 * 6, // 6 hours in milliseconds
		/**
		 * A flag that shows if minification is enabled or not
		 */
		Minify: true,
		/**
		 * The path of the abiword executable
		 */
		Abiword: nil,
		/**
		 * The path of the libreoffice executable
		 */
		SOffice: nil,
		/**
		 * Should we support none natively supported file types on import?
		 */
		AllowUnknownFileEnds: true,
		/**
		 * The log level of log4js
		 */
		LogLevel: "INFO",
		/**
		 * Disable IP logging
		 */
		DisableIPLogging: false,
		/**
		 * Number of seconds to automatically reconnect pad
		 */
		AutomaticReconnectionTimeout: 0,
		/**
		 * Disable Load Testing
		 */
		LoadTest: false,
		/**
		 * Disable dump of objects preventing a clean exit
		 */
		DumpOnCleanExit: false,
		/**
		 * Enable indentation on new lines
		 */
		IndentationOnNewLine: true,
		/*
		 * Trust Proxy, whether or not trust the x-forwarded-for header.
		 */
		TrustProxy: false,
		Cookie: Cookie{
			KeyRotationInterval:    1 * 24 * 60 * 60 * 1000,
			SameSite:               "lax",
			SessionLifetime:        10 * 24 * 60 * 60 * 1000,
			SessionRefreshInterval: 1 * 24 * 60 * 60 * 1000,
		},
		/*
		 * This setting is used if you need authentication and/or
		 * authorization. Note: /admin always requires authentication, and
		 * either authorization by a module, or a user with is_admin set
		 */
		RequireAuthentication: false,
		RequireAuthorization:  false,
		Users:                 make(map[string]User),
		/*
		 * This setting is used for configuring sso
		 */
		SSO: &SSO{
			Issuer: "http://localhost:9001",
		},
		/*
		 * Show settings in admin page, by default it is true
		 */
		ShowSettingsInAdminPage: true,
		Cleanup: Cleanup{
			Enabled:       false,
			KeepRevisions: 100,
		},
		ScrollWhenFocusLineIsOutOfViewport: struct {
			Percentage                               clientVars2.ScrollWhenFocusLineIsOutOfViewportPercentage `json:"percentage"`
			Duration                                 int                                                      `json:"duration"`
			ScrollWhenCaretIsInTheLastLineOfViewport bool                                                     `json:"scrollWhenCaretIsInTheLastLineOfViewport"`
			PercentageToScrollWhenUserPressesArrowUp int                                                      `json:"percentageToScrollWhenUserPressesArrowUp"`
		}{Percentage: clientVars2.ScrollWhenFocusLineIsOutOfViewportPercentage{
			EditionAboveViewport: 0,
			EditionBelowViewport: 0,
		}, Duration: 0, ScrollWhenCaretIsInTheLastLineOfViewport: false, PercentageToScrollWhenUserPressesArrowUp: 0},
		ExposeVersion:       false,
		CustomLocaleStrings: make(map[string]map[string]string),
		ImportExportRateLimiting: ImportExportRateLimiting{
			WindowMS: 90000,
			Max:      10,
		},
		CommitRateLimiting: CommitRateLimiting{
			Duration: 1,
			Points:   10,
		},
		ImportMaxFileSize:   50 * 1024 * 1024, // 50 MB
		EnableAdminUITests:  false,
		LowerCasePadIDs:     false,
		RandomVersionString: "123",
		GitVersion:          GetGitCommit(),
	}
}

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

// Merge merges non-zero values from src into dest, including nested structs, slices, and maps.
func Merge(dest, src interface{}) {
	destVal := reflect.ValueOf(dest).Elem()
	srcVal := reflect.ValueOf(src).Elem()

	for i := 0; i < srcVal.NumField(); i++ {
		srcField := srcVal.Field(i)
		destField := destVal.Field(i)

		switch srcField.Kind() {
		case reflect.Struct:
			Merge(destField.Addr().Interface(), srcField.Addr().Interface())
		case reflect.Slice:
			if !srcField.IsNil() {
				destField.Set(reflect.AppendSlice(destField, srcField))
			}
		case reflect.Map:
			if !srcField.IsNil() {
				if destField.IsNil() {
					destField.Set(reflect.MakeMap(destField.Type()))
				}
				for _, key := range srcField.MapKeys() {
					destField.SetMapIndex(key, srcField.MapIndex(key))
				}
			}
		default:
			if !isZero(srcField) {
				destField.Set(srcField)
			}
		}
	}
}

var envVarRe = regexp.MustCompile(`^\$\{([^:}]*)(:(.*))?\}$`)

// LookUpEnvVariables traversiert s und ersetzt String-Felder vom Format ${ENV} oder ${ENV:default}.
// - Falls ENV gesetzt: Wert aus der Umgebung verwenden.
// - Falls ENV nicht gesetzt und default vorhanden: default verwenden.
// - Sonst Originalwert belassen.
func LookUpEnvVariables(s *Settings) {
	if s == nil {
		return
	}
	processValue(reflect.ValueOf(s).Elem())
}

func processValue(v reflect.Value) {
	// If it's invalid, nothing to do
	if !v.IsValid() {
		return
	}

	switch v.Kind() {
	case reflect.Ptr:
		if v.IsNil() {
			return
		}
		processValue(v.Elem())
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		processValue(v.Elem())
	case reflect.Struct:
		// iterate fields; only addressable/exported fields can be Set
		for i := 0; i < v.NumField(); i++ {
			f := v.Field(i)
			// allow descending into unexported struct fields only if addressable (rare)
			if !f.CanSet() && !(f.Kind() == reflect.Struct || f.Kind() == reflect.Ptr || f.Kind() == reflect.Interface) {
				continue
			}
			processValue(f)
		}
	case reflect.Map:
		// iterate keys and replace values in-place
		for _, k := range v.MapKeys() {
			val := v.MapIndex(k)
			if !val.IsValid() {
				continue
			}
			// copy the value so we can modify it
			newVal := reflect.New(val.Type()).Elem()
			newVal.Set(val)
			processValue(newVal)
			v.SetMapIndex(k, newVal)
		}
	case reflect.Slice, reflect.Array:
		for i := 0; i < v.Len(); i++ {
			elem := v.Index(i)
			processValue(elem)
		}
	case reflect.String:
		orig := v.String()
		m := envVarRe.FindStringSubmatch(orig)
		if m == nil {
			return
		}
		envName := m[1]
		def := ""
		if len(m) >= 4 {
			def = m[3]
		}
		if envVal, ok := os.LookupEnv(envName); ok {
			v.SetString(envVal)
		} else if def != "" {
			v.SetString(def)
		}
	case reflect.Bool,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64:
		return
	default:
		return
	}
}

// isZero reports whether v is the zero value for its type.
func isZero(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.Len() == 0
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Interface, reflect.Ptr:
		return v.IsNil()
	case reflect.Slice, reflect.Map:
		return v.Len() == 0
	case reflect.Invalid:
		return true
	}
	return false
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

	if err != nil {
		println("Error reading settings. Default settings will be used.")
	}

	setting, err := ReadConfig(string(settings))
	if err != nil {
		println("error is" + err.Error())
		return
	}
	setting.GitVersion = GitVersion()
	setting.Root = pathToRoot
	Displayed = *setting

}
