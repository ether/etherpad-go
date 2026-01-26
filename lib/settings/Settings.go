package settings

import (
	"os"
	"path/filepath"

	clientVars2 "github.com/ether/etherpad-go/lib/models/clientVars"
	"go.uber.org/zap"
)

type DBSettings struct {
	Filename string `mapstructure:"filename"`
	Host     string `mapstructure:"host"`
	Port     string `mapstructure:"port"`
	Database string `mapstructure:"database"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	Charset  string `mapstructure:"charset"`
}

type TTL struct {
	AccessToken       int `mapstructure:"accessToken"`
	AuthorizationCode int `mapstructure:"authorizationCode"`
	ClientCredentials int `mapstructure:"clientCredentials"`
	IdToken           int `mapstructure:"idToken"`
	RefreshToken      int `mapstructure:"refreshToken"`
}

type PadOptions struct {
	NoColors         bool    `mapstructure:"noColors"`
	ShowControls     bool    `mapstructure:"showControls"`
	ShowChat         bool    `mapstructure:"showChat"`
	ShowLineNumbers  bool    `mapstructure:"showLineNumbers"`
	UseMonospaceFont bool    `mapstructure:"useMonospaceFont"`
	UserName         *bool   `mapstructure:"userName"`
	UserColor        *bool   `mapstructure:"userColor"`
	RTL              bool    `mapstructure:"rtl"`
	AlwaysShowChat   bool    `mapstructure:"alwaysShowChat"`
	ChatAndUsers     bool    `mapstructure:"chatAndUsers"`
	Lang             *string `mapstructure:"lang"`
}

type PadShortcutEnabled struct {
	AltF9     bool `json:"altF9" mapstructure:"altF9"`
	AltC      bool `json:"altC" mapstructure:"altC"`
	Delete    bool `json:"delete" mapstructure:"delete"`
	CmdShift2 bool `json:"cmdShift2" mapstructure:"cmdShift2"`
	Return    bool `json:"return" mapstructure:"return"`
	Esc       bool `json:"esc" mapstructure:"esc"`
	CmdS      bool `json:"cmdS" mapstructure:"cmdS"`
	Tab       bool `json:"tab" mapstructure:"tab"`
	CmdZ      bool `json:"cmdZ" mapstructure:"cmdZ"`
	CmdY      bool `json:"cmdY" mapstructure:"cmdY"`
	CmdB      bool `json:"cmdB" mapstructure:"cmdB"`
	CmdI      bool `json:"cmdI" mapstructure:"cmdI"`
	CmdU      bool `json:"cmdU" mapstructure:"cmdU"`
	Cmd5      bool `json:"cmd5" mapstructure:"cmd5"`
	CmdShiftL bool `json:"cmdShiftL" mapstructure:"cmdShiftL"`
	CmdShiftN bool `json:"cmdShiftN" mapstructure:"cmdShiftN"`
	CmdShift1 bool `json:"cmdShift1" mapstructure:"cmdShift1"`
	CmdShiftC bool `json:"cmdShiftC" mapstructure:"cmdShiftC"`
	CmdH      bool `json:"cmdH" mapstructure:"cmdH"`
	CtrlHome  bool `json:"ctrlHome" mapstructure:"ctrlHome"`
	PageUp    bool `json:"pageUp" mapstructure:"pageUp"`
	PageDown  bool `json:"pageDown" mapstructure:"pageDown"`
}

type Toolbar struct {
	Left       [][]string `mapstructure:"left"`
	Right      [][]string `mapstructure:"right"`
	TimeSlider [][]string `mapstructure:"timeSlider"`
}

type User struct {
	Password *string `json:"password" mapstructure:"password"`
	IsAdmin  *bool   `json:"is_admin" mapstructure:"is_admin"`
	Username *string `json:"username" mapstructure:"username"`
}

type Cookie struct {
	KeyRotationInterval    int64  `json:"keyRotationInterval" mapstructure:"keyRotationInterval"`
	SameSite               string `json:"sameSite" mapstructure:"sameSite"`
	SessionLifetime        int64  `json:"sessionLifetime" mapstructure:"sessionLifetime"`
	SessionRefreshInterval int64  `json:"sessionRefreshInterval" mapstructure:"sessionRefreshInterval"`
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
	Issuer  string      `json:"issuer" mapstructure:"issuer"`
	Clients []SSOClient `json:"clients" mapstructure:"clients"`
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
	Enabled       bool `json:"enabled" mapstructure:"enabled"`
	KeepRevisions int  `json:"keepRevisions" mapstructure:"keepRevisions"`
}

type ImportExportRateLimiting struct {
	WindowMS int `json:"windowMS" mapstructure:"windowMs"`
	Max      int `json:"max" mapstructure:"max"`
}

type CommitRateLimiting struct {
	Duration int  `json:"duration" mapstructure:"duration"`
	Points   int  `json:"points" mapstructure:"points"`
	LoadTest bool `json:"loadTest" mapstructure:"loadTest"`
}

type SSLSettings struct {
	Key  string   `json:"key" mapstructure:"key"`
	Cert string   `json:"cert" mapstructure:"cert"`
	Ca   []string `json:"ca" mapstructure:"ca"`
}

// PluginSettings definiert die Einstellungen für einzelne Plugins
type PluginSettings struct {
	Enabled bool `json:"enabled" mapstructure:"enabled"`
}

type Settings struct {
	Title          string  `json:"title" mapstructure:"title"`
	ShowRecentPads bool    `json:"showRecentPads" mapstructure:"showRecentPads"`
	Favicon        *string `json:"favicon" mapstructure:"favicon"`

	TTL            TTL    `json:"ttl" mapstructure:"ttl"`
	UpdateServer   string `json:"updateServer" mapstructure:"updateServer"`
	EnableDarkMode bool   `json:"enableDarkMode" mapstructure:"enableDarkMode"`

	SkinName     string `json:"skinName" mapstructure:"skinName"`
	SkinVariants string `json:"skinVariants" mapstructure:"skinVariants"`
	IP           string `json:"ip" mapstructure:"ip"`
	Port         string `json:"port" mapstructure:"port"`

	SuppressErrorsInPadText bool `json:"suppressErrorsInPadText" mapstructure:"suppressErrorsInPadText"`

	SSL        SSLSettings `json:"ssl" mapstructure:"ssl"`
	DBType     IDBType     `json:"dbType" mapstructure:"dbType"`
	DBSettings *DBSettings `json:"dbSettings" mapstructure:"dbSettings"`

	DefaultPadText string `json:"defaultPadText" mapstructure:"defaultPadText"`

	PadOptions         PadOptions         `json:"padOptions" mapstructure:"padOptions"`
	PadShortcutEnabled PadShortcutEnabled `json:"padShortcutEnabled" mapstructure:"padShortcutEnabled"`

	EnableMetrics bool `json:"enableMetrics" mapstructure:"enableMetrics"`

	RequireSession bool `json:"requireSession" mapstructure:"requireSession"`
	EditOnly       bool `json:"editOnly" mapstructure:"editOnly"`
	MaxAge         int  `json:"maxAge" mapstructure:"maxAge"`
	Minify         bool `json:"minify" mapstructure:"minify"`

	AllowUnknownFileEnds bool `json:"allowUnknownFileEnds" mapstructure:"allowUnknownFileEnds"`

	LogLevel                     string `json:"loglevel" mapstructure:"loglevel"`
	DisableIPLogging             bool   `json:"disableIPLogging" mapstructure:"disableIPLogging"`
	AutomaticReconnectionTimeout int    `json:"automaticReconnectionTimeout" mapstructure:"automaticReconnectionTimeout"`

	LoadTest        bool `json:"loadTest" mapstructure:"loadTest"`
	DumpOnCleanExit bool `json:"dumpOnUncleanExit" mapstructure:"dumpOnUncleanExit"`

	TrustProxy bool `json:"trustProxy" mapstructure:"trustProxy"`

	Cookie Cookie `json:"cookie" mapstructure:"cookie"`

	RequireAuthentication bool `json:"requireAuthentication" mapstructure:"requireAuthentication"`
	RequireAuthorization  bool `json:"requireAuthorization" mapstructure:"requireAuthorization"`

	Users map[string]User `json:"users" mapstructure:"users"`

	ShowSettingsInAdminPage bool `json:"showSettingsInAdminPage" mapstructure:"showSettingsInAdminPage"`

	ScrollWhenFocusLineIsOutOfViewport clientVars2.ScrollWhenFocusLineIsOutOfViewport `json:"scrollWhenFocusLineIsOutOfViewport" mapstructure:"scrollWhenFocusLineIsOutOfViewport"`

	SocketIo SocketIoSettings `json:"socketIo" mapstructure:"socketIo"`

	AuthenticationMethod string `json:"authenticationMethod" mapstructure:"authenticationMethod"`

	SSO *SSO `json:"sso" mapstructure:"sso"`

	Toolbar Toolbar `json:"toolbar" mapstructure:"toolbar"`

	Cleanup Cleanup `json:"cleanup" mapstructure:"cleanup"`

	ExposeVersion bool `json:"exposeVersion" mapstructure:"exposeVersion"`

	CustomLocaleStrings map[string]map[string]string `json:"customLocaleStrings" mapstructure:"customLocaleStrings"`

	ImportExportRateLimiting ImportExportRateLimiting `json:"importExportRateLimiting" mapstructure:"importExportRateLimiting"`
	CommitRateLimiting       CommitRateLimiting       `json:"commitRateLimiting" mapstructure:"commitRateLimiting"`

	ImportMaxFileSize  int64 `json:"importMaxFileSize" mapstructure:"importMaxFileSize"`
	EnableAdminUITests bool  `json:"enableAdminUITests" mapstructure:"enableAdminUITests"`
	LowerCasePadIDs    bool  `json:"lowerCasePadIds" mapstructure:"lowerCasePadIds"`

	DevMode bool `json:"devMode" mapstructure:"devMode"`

	AvailableExports     []string `json:"availableExports" mapstructure:"availableExports"`
	IndentationOnNewLine bool     `json:"indentationOnNewLine" mapstructure:"indentationOnNewLine"`

	Plugins map[string]PluginSettings `json:"plugins" mapstructure:"plugins"`

	// Untracked fields
	Root                string `json:"-"`
	GitVersion          string `json:"-"`
	RandomVersionString string `json:"-"`
}

// IsPluginEnabled prüft, ob ein Plugin in den Settings aktiviert ist
func (s *Settings) IsPluginEnabled(pluginName string) bool {
	if s.Plugins == nil {
		return false
	}
	if plugin, exists := s.Plugins[pluginName]; exists {
		return plugin.Enabled
	}
	return false
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

type SocketIoSettings struct {
	MaxHttpBufferSize int64 `json:"maxHttpBufferSize"`
}

var Displayed Settings

type Options struct {
	Whitespace     bool
	TrailingCommas bool
}

func InitSettings(logger *zap.SugaredLogger) {
	var pathToRoot string

	var envPathToSettings = os.Getenv("ETHERPAD_SETTINGS_PATH")
	if envPathToSettings != "" {
		pathToRoot = envPathToSettings
	}

	if pathToRoot == "" {
		execPath, err := os.Executable()
		if err != nil {
			panic("Error finding executable path: " + err.Error())
		}
		pathToRoot = filepath.Dir(execPath)

		assetsPath := filepath.Join(pathToRoot, "assets")
		if _, err := os.Stat(assetsPath); os.IsNotExist(err) {
			wd, err := os.Getwd()
			if err == nil {
				if _, err := os.Stat(filepath.Join(wd, "assets")); err == nil {
					pathToRoot = wd
				}
			}
		}
	}

	setting, err := ReadConfig()
	if err != nil {
		logger.Errorf("Error during reading the config " + err.Error())
		return
	}

	if setting.DBSettings != nil && setting.DBSettings.Filename != "" {
		dbDir := filepath.Dir(setting.DBSettings.Filename)
		if dbDir != "" && dbDir != "." {
			if err := os.MkdirAll(dbDir, 0755); err != nil {
				panic("Error creating database directory: " + err.Error())
			}
		}
	}

	setting.GitVersion = GitVersion()
	setting.Root = pathToRoot
	Displayed = *setting

}
