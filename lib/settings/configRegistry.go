package settings

import (
	"strings"

	"github.com/spf13/viper"
)

type ConfigKey struct {
	Key         string
	Default     any
	Description string
}

const envPrefix = "ETHERPAD"

func EnvVar(key string) string {
	return envPrefix + "_" + strings.ToUpper(
		strings.ReplaceAll(key, ".", "_"),
	)
}

var Registry = []ConfigKey{
	// ---------------------------------------------------------------------
	// Core
	// ---------------------------------------------------------------------
	{Key: Title, Default: "Etherpad", Description: "Application title"},
	{Key: ShowRecentPads, Default: true, Description: "Show recent pads"},
	{Key: Favicon, Default: nil, Description: "Custom favicon"},
	{Key: Skinname, Default: "colibris", Description: "UI skin name"},
	{
		Key:         SkinVariants,
		Default:     "super-light-toolbar super-light-editor light-background",
		Description: "Skin variants",
	},
	{Key: IP, Default: "0.0.0.0", Description: "Bind address"},
	{Key: Port, Default: "9001", Description: "HTTP server port"},
	{
		Key:         ShowSettingsInAdminPage,
		Default:     true,
		Description: "Show settings in admin UI",
	},
	{
		Key:         SuppressErrorsInPadText,
		Default:     false,
		Description: "Suppress errors in pad text",
	},
	{
		Key:         SocketIoMaxHttpBufferSize,
		Default:     50000,
		Description: "Socket.IO max HTTP buffer size",
	},
	{
		Key:         AuthenticationMethod,
		Default:     "sso",
		Description: "Authentication method",
	},

	// ---------------------------------------------------------------------
	// Database
	// ---------------------------------------------------------------------
	{Key: DBType, Default: SQLITE, Description: "Database type"},
	{Key: DBSettingsHost, Default: nil, Description: "Database host"},
	{Key: DBSettingsUser, Default: nil, Description: "Database user"},
	{Key: DBSettingsPassword, Default: nil, Description: "Database password"},
	{Key: DBSettingsDatabase, Default: nil, Description: "Database name"},
	{Key: DBSettingsPort, Default: nil, Description: "Database port"},
	{Key: DBSettingsCharset, Default: "utf8mb4", Description: "Database charset (only relevant for MySQL)"},
	{
		Key:         DBSettingsFilename,
		Default:     "var/etherpad.db",
		Description: "SQLite database filename",
	},

	// ---------------------------------------------------------------------
	// Pad defaults
	// ---------------------------------------------------------------------
	{
		Key:         DefaultPadText,
		Default:     "Welcome to Etherpad!...",
		Description: "Default pad text",
	},
	{Key: PadOptionsNoColors, Default: false, Description: "Disable colors"},
	{Key: PadOptionsShowControls, Default: true, Description: "Show controls"},
	{Key: PadOptionsShowChat, Default: true, Description: "Show chat"},
	{Key: PadOptionsShowLineNumbers, Default: true, Description: "Show line numbers"},
	{Key: PadOptionsUseMonospaceFont, Default: false, Description: "Use monospace font"},
	{Key: PadOptionsUserName, Default: nil, Description: "Default username"},
	{Key: PadOptionsUserColor, Default: nil, Description: "Default user color"},
	{Key: PadOptionsRtl, Default: false, Description: "Right-to-left text"},
	{Key: PadOptionsAlwaysShowChat, Default: false, Description: "Always show chat"},
	{Key: PadOptionsChatAndUsers, Default: false, Description: "Chat and users list"},
	{Key: PadOptionsLang, Default: "en-gb", Description: "Pad language"},

	// ---------------------------------------------------------------------
	// Shortcuts
	// ---------------------------------------------------------------------
	{Key: PadShortcutEnabledAltF9, Default: true, Description: "Enable Alt+F9"},
	{Key: PadShortcutEnabledAltC, Default: true, Description: "Enable Alt+C"},
	{Key: PadShortcutEnabledDelete, Default: true, Description: "Enable Delete"},
	{Key: PadShortcutEnabledCmdShift2, Default: true, Description: "Enable Cmd+Shift+2"},
	{Key: PadShortcutEnabledReturn, Default: true, Description: "Enable Return"},
	{Key: PadShortcutEnabledEsc, Default: true, Description: "Enable Escape"},
	{Key: PadShortcutEnabledCmdS, Default: true, Description: "Enable Cmd+S"},
	{Key: PadShortcutEnabledTab, Default: true, Description: "Enable Tab"},
	{Key: PadShortcutEnabledCmdZ, Default: true, Description: "Enable Cmd+Z"},
	{Key: PadShortcutEnabledCmdY, Default: true, Description: "Enable Cmd+Y"},
	{Key: PadShortcutEnabledCmdB, Default: true, Description: "Enable Cmd+B"},
	{Key: PadShortcutEnabledCmdI, Default: true, Description: "Enable Cmd+I"},
	{Key: PadShortcutEnabledCmdU, Default: true, Description: "Enable Cmd+U"},
	{Key: PadShortcutEnabledCmd5, Default: true, Description: "Enable Cmd+5"},
	{Key: PadShortcutEnabledCmdShiftL, Default: true, Description: "Enable Cmd+Shift+L"},
	{Key: PadShortcutEnabledCmdShiftN, Default: true, Description: "Enable Cmd+Shift+N"},
	{Key: PadShortcutEnabledCmdShift1, Default: true, Description: "Enable Cmd+Shift+1"},
	{Key: PadShortcutEnabledCmdShiftC, Default: true, Description: "Enable Cmd+Shift+C"},
	{Key: PadShortcutEnabledCmdH, Default: true, Description: "Enable Cmd+H"},
	{Key: PadShortcutEnabledCtrlHome, Default: true, Description: "Enable Ctrl+Home"},
	{Key: PadShortcutEnabledPageUp, Default: true, Description: "Enable PageUp"},
	{Key: PadShortcutEnabledPageDown, Default: true, Description: "Enable PageDown"},

	// ---------------------------------------------------------------------
	// Misc / runtime
	// ---------------------------------------------------------------------
	{Key: EnableMetrics, Default: true, Description: "Enable metrics"},
	{Key: CleanupExpr, Default: true, Description: "Enable cleanup expressions"},
	{Key: RequireSession, Default: false, Description: "Require session"},
	{Key: EditOnly, Default: false, Description: "Edit-only mode"},
	{Key: MaxAge, Default: 1000 * 60 * 60 * 6, Description: "Session max age"},
	{Key: Abiword, Default: nil, Description: "Abiword path"},
	{Key: Soffice, Default: nil, Description: "LibreOffice path"},
	{Key: Minify, Default: true, Description: "Minify assets"},
	{Key: AllowUnknownFileEnds, Default: true, Description: "Allow unknown file extensions"},
	{Key: Loglevel, Default: "INFO", Description: "Log level"},
	{
		Key:         CustomLocaleStrings,
		Default:     map[string]map[string]string{},
		Description: "Custom locale strings",
	},
	{Key: DisableIPlogging, Default: false, Description: "Disable IP logging"},
	{
		Key:         AutomaticReconnectionTimeout,
		Default:     0,
		Description: "Automatic reconnection timeout",
	},

	// ---------------------------------------------------------------------
	// Scroll behavior
	// ---------------------------------------------------------------------
	{Key: ScrollWhenFocusPercentage, Default: 0, Description: "Scroll percentage"},
	{
		Key:         ScrollWhenFocusEditionAboveViewport,
		Default:     0,
		Description: "Scroll when editing above viewport",
	},
	{
		Key:         ScrollWhenFocusEditionBelowViewport,
		Default:     0,
		Description: "Scroll when editing below viewport",
	},
	{Key: ScrollWhenFocusDuration, Default: 0, Description: "Scroll duration"},
	{
		Key:         ScrollWhenFocusCaretScroll,
		Default:     false,
		Description: "Scroll caret into view",
	},
	{
		Key:         ScrollWhenFocusPercentageArrowUp,
		Default:     0,
		Description: "Scroll percentage on arrow up",
	},

	// ---------------------------------------------------------------------
	// Users / auth / security
	// ---------------------------------------------------------------------
	{Key: Users, Default: map[string]User{}, Description: "Static users"},
	{Key: LoadTest, Default: false, Description: "Load test mode"},
	{Key: DumpOnUncleanExit, Default: false, Description: "Dump state on crash"},
	{Key: TrustProxy, Default: false, Description: "Trust reverse proxy"},
	{
		Key:         CookieKeyRotationInterval,
		Default:     1 * 24 * 60 * 60 * 1000,
		Description: "Cookie key rotation interval",
	},
	{Key: CookieSameSite, Default: "lax", Description: "Cookie SameSite policy"},
	{
		Key:         CookieSessionLifetime,
		Default:     10 * 24 * 60 * 60 * 1000,
		Description: "Cookie session lifetime",
	},
	{
		Key:         CookieSessionRefreshInterval,
		Default:     1 * 24 * 60 * 60 * 1000,
		Description: "Cookie session refresh interval",
	},
	{Key: RequireAuthentication, Default: false, Description: "Require authentication"},
	{Key: RequireAuthorization, Default: false, Description: "Require authorization"},
	{Key: SsoIssuer, Default: "http://localhost:3000", Description: "SSO issuer"},
	{
		Key:         SsoClients,
		Default:     map[string]SSOClient{},
		Description: "SSO clients",
	},

	// ---------------------------------------------------------------------
	// Cleanup / limits / exports
	// ---------------------------------------------------------------------
	{Key: CleanupEnabled, Default: false, Description: "Enable cleanup"},
	{
		Key:         CleanupKeepRevisions,
		Default:     100,
		Description: "Revisions to keep",
	},
	{Key: ExposeVersion, Default: false, Description: "Expose version"},
	{
		Key:         ImportExportRateLimitingWindowMs,
		Default:     90000,
		Description: "Import/export rate limit window",
	},
	{
		Key:         ImportExportRateLimitingMax,
		Default:     10,
		Description: "Import/export rate limit max",
	},
	{
		Key:         CommitRateLimitingDuration,
		Default:     1,
		Description: "Commit rate limit duration",
	},
	{
		Key:         CommitRateLimitingPoints,
		Default:     10,
		Description: "Commit rate limit points",
	},
	{
		Key:         ImportMaxFileSize,
		Default:     50 * 1024 * 1024,
		Description: "Max import file size",
	},
	{
		Key:         EnableAdminUITests,
		Default:     false,
		Description: "Enable admin UI tests",
	},
	{
		Key:         LowerCasePadIds,
		Default:     false,
		Description: "Force lowercase pad IDs",
	},
	{
		Key:         UpdateServer,
		Default:     "https://static.etherpad.org",
		Description: "Update server",
	},
	{
		Key:         EnableDarkMode,
		Default:     true,
		Description: "Enable dark mode",
	},
	{
		Key:         AvailableExports,
		Default:     []string{"txt", "pdf", "etherpad", "word", "open", "html", "markdown"},
		Description: "Available export formats",
	},
	{
		Key:         IndentationOnNewLine,
		Default:     true,
		Description: "Enable automatic indentation on new lines",
	},

	// ---------------------------------------------------------------------
	// Plugins
	// ---------------------------------------------------------------------
	{Key: EpAlignEnabled, Default: false, Description: "Enable ep_align plugin"},
	{
		Key:         EpSpellcheckEnabled,
		Default:     false,
		Description: "Enable ep_spellcheck plugin",
	},
	{
		Key:         EpMarkdownEnabled,
		Default:     false,
		Description: "Enable ep_markdown plugin",
	},
}

func ApplyRegistryDefaults() {
	for _, c := range Registry {
		viper.SetDefault(c.Key, c.Default)
	}
}
