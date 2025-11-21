package settings

import (
	"errors"
	"strings"

	clientVars2 "github.com/ether/etherpad-go/lib/models/clientVars"
	"github.com/spf13/viper"
)

func ReadConfig(jsonStr string) (*Settings, error) {
	viper.SetConfigName("settings")
	viper.SetConfigType("json")

	viper.AddConfigPath(".")
	viper.AutomaticEnv()
	viper.SetEnvPrefix("etherpad")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	if jsonStr != "" {
		if err := viper.ReadConfig(strings.NewReader(jsonStr)); err != nil {
			return nil, err
		}
	} else {
		if err := viper.ReadInConfig(); err != nil {
			var configFileNotFoundError viper.ConfigFileNotFoundError
			if !errors.As(err, &configFileNotFoundError) {
				return nil, err
			}
			// Datei nicht gefunden ist OK, fahre mit Defaults fort
		}
	}
	var favicon *string
	if faviconValue := viper.GetString(Favicon); faviconValue != "" {
		favicon = &faviconValue
	}

	viper.SetDefault(Title, "Etherpad")
	viper.SetDefault(ShowRecentPads, true)
	viper.SetDefault(Favicon, nil)
	viper.SetDefault(Skinname, "colibris")
	viper.SetDefault(SkinVariants, "super-light-toolbar super-light-editor light-background")
	viper.SetDefault(IP, "0.0.0.0")
	viper.SetDefault(Port, "9001")
	viper.SetDefault(ShowSettingsInAdminPage, true)
	viper.SetDefault(SuppressErrorsInPadText, false)
	viper.SetDefault(SocketIoMaxHttpBufferSize, 50000)
	viper.SetDefault(AuthenticationMethod, "sso")

	viper.SetDefault(DBType, SQLITE)
	viper.SetDefault(DBSettingsHost, nil)
	viper.SetDefault(DBSettingsUser, nil)
	viper.SetDefault(DBSettingsPassword, nil)
	viper.SetDefault(DBSettingsDatabase, nil)
	viper.SetDefault(DBSettingsPort, nil)
	viper.SetDefault(DBSettingsCharset, "utf8mb4")
	viper.SetDefault(DBSettingsFilename, "var/etherpad.db")
	viper.SetDefault(DBSettingsCollection, "etherpad")
	viper.SetDefault(DBSettingsURL, "mongodb://localhost:27017/etherpad")

	viper.SetDefault(DefaultPadText, "Welcome to Etherpad!\n\nThis pad text is synchronized as you type, so that everyone viewing this page sees the same text. This allows you to collaborate seamlessly on documents!\n\nEtherpad on Github: https://github.com/ether/etherpad-lite")
	viper.SetDefault(PadOptionsNoColors, false)
	viper.SetDefault(PadOptionsShowControls, true)
	viper.SetDefault(PadOptionsShowChat, true)
	viper.SetDefault(PadOptionsShowLineNumbers, true)
	viper.SetDefault(PadOptionsUseMonospaceFont, false)
	viper.SetDefault(PadOptionsUserName, nil)
	viper.SetDefault(PadOptionsUserColor, nil)
	viper.SetDefault(PadOptionsRtl, false)
	viper.SetDefault(PadOptionsAlwaysShowChat, false)
	viper.SetDefault(PadOptionsChatAndUsers, false)
	viper.SetDefault(PadOptionsLang, "en-gb")

	viper.SetDefault(PadShortcutEnabledAltF9, true)
	viper.SetDefault(PadShortcutEnabledAltC, true)
	viper.SetDefault(PadShortcutEnabledDelete, true)
	viper.SetDefault(PadShortcutEnabledCmdShift2, true)
	viper.SetDefault(PadShortcutEnabledReturn, true)
	viper.SetDefault(PadShortcutEnabledEsc, true)
	viper.SetDefault(PadShortcutEnabledCmdS, true)
	viper.SetDefault(PadShortcutEnabledTab, true)
	viper.SetDefault(PadShortcutEnabledCmdZ, true)
	viper.SetDefault(PadShortcutEnabledCmdY, true)
	viper.SetDefault(PadShortcutEnabledCmdB, true)
	viper.SetDefault(PadShortcutEnabledCmdI, true)
	viper.SetDefault(PadShortcutEnabledCmdU, true)
	viper.SetDefault(PadShortcutEnabledCmd5, true)
	viper.SetDefault(PadShortcutEnabledCmdShiftL, true)
	viper.SetDefault(PadShortcutEnabledCmdShiftN, true)
	viper.SetDefault(PadShortcutEnabledCmdShift1, true)
	viper.SetDefault(PadShortcutEnabledCmdShiftC, true)
	viper.SetDefault(PadShortcutEnabledCmdH, true)
	viper.SetDefault(PadShortcutEnabledCtrlHome, true)
	viper.SetDefault(PadShortcutEnabledPageUp, true)
	viper.SetDefault(PadShortcutEnabledPageDown, true)

	viper.SetDefault(EnableMetrics, true)
	viper.SetDefault(CleanupExpr, true)

	viper.SetDefault(RequireSession, false)
	viper.SetDefault(EditOnly, false)
	viper.SetDefault(MaxAge, 1000*60*60*6)
	viper.SetDefault(Abiword, nil)
	viper.SetDefault(Soffice, nil)
	viper.SetDefault(Minify, true)
	viper.SetDefault(AllowUnknownFileEnds, true)
	viper.SetDefault(Loglevel, "INFO")
	viper.SetDefault(CustomLocaleStrings, make(map[string]map[string]string))

	viper.SetDefault(DisableIPlogging, false)
	viper.SetDefault(AutomaticReconnectionTimeout, 0)

	viper.SetDefault(ScrollWhenFocusPercentage, 0)
	viper.SetDefault(ScrollWhenFocusEditionAboveViewport, 0)
	viper.SetDefault(ScrollWhenFocusEditionBelowViewport, 0)
	viper.SetDefault(ScrollWhenFocusDuration, 0)
	viper.SetDefault(ScrollWhenFocusCaretScroll, false)

	viper.SetDefault(Users, make(map[string]User))

	viper.SetDefault(LoadTest, false)
	viper.SetDefault(DumpOnUncleanExit, false)
	viper.SetDefault(TrustProxy, false)

	viper.SetDefault(CookieKeyRotationInterval, 1*24*60*60*1000)
	viper.SetDefault(CookieSameSite, "lax")
	viper.SetDefault(CookieSessionLifetime, 10*24*60*60*1000)
	viper.SetDefault(CookieSessionRefreshInterval, 1*24*60*60*1000)
	viper.SetDefault(RequireAuthentication, false)
	viper.SetDefault(RequireAuthorization, false)
	viper.SetDefault(SsoIssuer, "http://localhost:3000")
	viper.SetDefault(CleanupEnabled, false)
	viper.SetDefault(CleanupKeepRevisions, 100)
	viper.SetDefault(SsoClients, make(map[string]SSOClient))

	viper.SetDefault(ScrollWhenFocusPercentageArrowUp, 0)
	viper.SetDefault(ExposeVersion, false)
	viper.SetDefault(ImportExportRateLimitingWindowMs, 90000)
	viper.SetDefault(ImportExportRateLimitingMax, 10)
	viper.SetDefault(CommitRateLimitingDuration, 1)
	viper.SetDefault(CommitRateLimitingPoints, 10)
	viper.SetDefault(ImportMaxFileSize, 50*1024*1024)
	viper.SetDefault(EnableAdminUITests, false)
	viper.SetDefault(LowerCasePadIds, false)
	viper.SetDefault(UpdateServer, "https://static.etherpad.org")
	viper.SetDefault(EnableDarkMode, true)

	users := make(map[string]User)
	if err := viper.UnmarshalKey(Users, &users); err != nil {
		users = make(map[string]User)
	}

	customLocaleStrings := make(map[string]map[string]string)
	if raw := viper.Get(CustomLocaleStrings); raw != nil {
		if converted, ok := raw.(map[string]map[string]string); ok {
			customLocaleStrings = converted
		}
	}

	var ssoClients []SSOClient
	if err := viper.UnmarshalKey(SsoClients, &ssoClients); err != nil || ssoClients == nil {
		ssoClients = make([]SSOClient, 0)
	}

	dbTypeToUse, err := ParseDBType(viper.GetString(DBType))
	if err != nil {
		return nil, err
	}

	s := &Settings{
		Title:          viper.GetString(Title),
		ShowRecentPads: viper.GetBool(ShowRecentPads),
		Favicon:        favicon,

		TTL: TTL{
			AccessToken:       viper.GetInt("ttl.accessToken"),
			AuthorizationCode: viper.GetInt("ttl.authorizationCode"),
			ClientCredentials: viper.GetInt("ttl.clientCredentials"),
			IdToken:           viper.GetInt("ttl.idToken"),
			RefreshToken:      viper.GetInt("ttl.refreshToken"),
		},

		UpdateServer:   viper.GetString(UpdateServer),
		EnableDarkMode: viper.GetBool(EnableDarkMode),

		SkinName:     viper.GetString(Skinname),
		SkinVariants: viper.GetString(SkinVariants),
		IP:           viper.GetString(IP),
		Port:         viper.GetString(Port),

		SuppressErrorsInPadText: viper.GetBool(SuppressErrorsInPadText),
		SSL: SSLSettings{
			Key:  viper.GetString(SSLKey),
			Cert: viper.GetString(SSLCert),
			Ca:   viper.GetStringSlice(SSLCa),
		},
		SocketIo: SocketIoSettings{
			MaxHttpBufferSize: viper.GetInt64(SocketIoMaxHttpBufferSize),
		},
		AuthenticationMethod: viper.GetString(AuthenticationMethod),
		DBType:               dbTypeToUse,
		DBSettings: &DBSettings{
			Host:       viper.GetString(DBSettingsHost),
			Port:       viper.GetString(DBSettingsPort),
			Database:   viper.GetString(DBSettingsDatabase),
			User:       viper.GetString(DBSettingsUser),
			Password:   viper.GetString(DBSettingsPassword),
			Charset:    viper.GetString(DBSettingsCharset),
			Filename:   viper.GetString(DBSettingsFilename),
			Collection: viper.GetString(DBSettingsCollection),
			Url:        viper.GetString(DBSettingsURL),
		},

		DefaultPadText: viper.GetString(DefaultPadText),

		PadOptions: PadOptions{
			NoColors:         viper.GetBool(PadOptionsNoColors),
			ShowControls:     viper.GetBool(PadOptionsShowControls),
			ShowChat:         viper.GetBool(PadOptionsShowChat),
			ShowLineNumbers:  viper.GetBool(PadOptionsShowLineNumbers),
			UseMonospaceFont: viper.GetBool(PadOptionsUseMonospaceFont),
			UserName:         nil,
			UserColor:        nil,
			RTL:              viper.GetBool(PadOptionsRtl),
			AlwaysShowChat:   viper.GetBool(PadOptionsAlwaysShowChat),
			ChatAndUsers:     viper.GetBool(PadOptionsChatAndUsers),
			Lang:             nil,
		},

		EnableMetrics: viper.GetBool(EnableMetrics),

		PadShortCutEnabled: PadShortcutEnabled{
			AltF9:     viper.GetBool(PadShortcutEnabledAltF9),
			AltC:      viper.GetBool(PadShortcutEnabledAltC),
			Delete:    viper.GetBool(PadShortcutEnabledDelete),
			CmdShift2: viper.GetBool(PadShortcutEnabledCmdShift2),
			Return:    viper.GetBool(PadShortcutEnabledReturn),
			Esc:       viper.GetBool(PadShortcutEnabledEsc),
			CmdS:      viper.GetBool(PadShortcutEnabledCmdS),
			Tab:       viper.GetBool(PadShortcutEnabledTab),
			CmdZ:      viper.GetBool(PadShortcutEnabledCmdZ),
			CmdY:      viper.GetBool(PadShortcutEnabledCmdY),
			CmdB:      viper.GetBool(PadShortcutEnabledCmdB),
			CmdI:      viper.GetBool(PadShortcutEnabledCmdI),
			CmdU:      viper.GetBool(PadShortcutEnabledCmdU),
			Cmd5:      viper.GetBool(PadShortcutEnabledCmd5),
			CmdShiftL: viper.GetBool(PadShortcutEnabledCmdShiftL),
			CmdShiftN: viper.GetBool(PadShortcutEnabledCmdShiftN),
			CmdShift1: viper.GetBool(PadShortcutEnabledCmdShift1),
			CmdShiftC: viper.GetBool(PadShortcutEnabledCmdShiftC),
			CmdH:      viper.GetBool(PadShortcutEnabledCmdH),
			CtrlHome:  viper.GetBool(PadShortcutEnabledCtrlHome),
			PageUp:    viper.GetBool(PadShortcutEnabledPageUp),
			PageDown:  viper.GetBool(PadShortcutEnabledPageDown),
		},

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

		RequireSession: viper.GetBool(RequireSession),
		EditOnly:       viper.GetBool(EditOnly),
		MaxAge:         viper.GetInt(MaxAge),
		Minify:         viper.GetBool(Minify),
		Abiword:        nil,
		SOffice:        nil,

		AllowUnknownFileEnds:         viper.GetBool(AllowUnknownFileEnds),
		LogLevel:                     viper.GetString(Loglevel),
		DisableIPLogging:             viper.GetBool(DisableIPlogging),
		AutomaticReconnectionTimeout: viper.GetInt(AutomaticReconnectionTimeout),
		LoadTest:                     viper.GetBool(LoadTest),
		DumpOnCleanExit:              viper.GetBool(DumpOnUncleanExit),
		IndentationOnNewLine:         true,
		TrustProxy:                   viper.GetBool(TrustProxy),

		Cookie: Cookie{
			KeyRotationInterval:    viper.GetInt64(CookieKeyRotationInterval),
			SameSite:               viper.GetString(CookieSameSite),
			SessionLifetime:        viper.GetInt64(CookieSessionLifetime),
			SessionRefreshInterval: viper.GetInt64(CookieSessionRefreshInterval),
		},

		RequireAuthentication: viper.GetBool(RequireAuthentication),
		RequireAuthorization:  viper.GetBool(RequireAuthorization),
		Users:                 users,

		SSO: &SSO{
			Issuer:  viper.GetString(SsoIssuer),
			Clients: ssoClients,
		},

		ShowSettingsInAdminPage: viper.GetBool(ShowSettingsInAdminPage),

		Cleanup: Cleanup{
			Enabled:       viper.GetBool(CleanupEnabled),
			KeepRevisions: viper.GetInt(CleanupKeepRevisions),
		},

		ScrollWhenFocusLineIsOutOfViewport: clientVars2.ScrollWhenFocusLineIsOutOfViewport{
			Percentage: clientVars2.ScrollWhenFocusLineIsOutOfViewportPercentage{
				EditionAboveViewport: viper.GetInt(ScrollWhenFocusEditionAboveViewport),
				EditionBelowViewport: viper.GetInt(ScrollWhenFocusEditionBelowViewport),
			},
			Duration:                                 viper.GetInt(ScrollWhenFocusDuration),
			ScrollWhenCaretIsInTheLastLineOfViewport: viper.GetBool(ScrollWhenFocusCaretScroll),
			PercentageToScrollWhenUserPressesArrowUp: viper.GetInt(ScrollWhenFocusPercentageArrowUp),
		},

		ExposeVersion:       viper.GetBool(ExposeVersion),
		CustomLocaleStrings: customLocaleStrings,

		ImportExportRateLimiting: ImportExportRateLimiting{
			WindowMS: viper.GetInt(ImportExportRateLimitingWindowMs),
			Max:      viper.GetInt(ImportExportRateLimitingMax),
		},

		CommitRateLimiting: CommitRateLimiting{
			Duration: viper.GetInt(CommitRateLimitingDuration),
			Points:   viper.GetInt(CommitRateLimitingPoints),
		},

		ImportMaxFileSize:   viper.GetInt64(ImportMaxFileSize),
		EnableAdminUITests:  viper.GetBool(EnableAdminUITests),
		LowerCasePadIDs:     viper.GetBool(LowerCasePadIds),
		RandomVersionString: "123",
		DevMode:             viper.GetBool(DevMode),
	}

	return s, nil
}
