package settings

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

func ReadConfig() (*Settings, error) {
	viper.Reset()

	viper.SetEnvPrefix("etherpad")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	ApplyRegistryDefaults()

	viper.SetConfigType("json")
	// settings.json may hold comments, trailing commas and ${VAR:default}
	// values, like etherpad-lite's; viper reads plain JSON.
	if raw, err := os.ReadFile("settings.json"); err == nil {
		prepared, err := PrepareSettingsJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("settings.json: %w", err)
		}
		if err := viper.ReadConfig(bytes.NewReader(prepared)); err != nil {
			return nil, err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}

	// Older settings files (and etherpad-lite's template) wrote
	// padOptions.userName/userColor = false for "no preset"; the options are
	// strings now (etherpad-lite's reloadSettings shim does the same).
	for _, key := range []string{PadOptionsUserName, PadOptionsUserColor} {
		if _, isBool := viper.Get(key).(bool); isBool {
			viper.Set(key, "")
		}
	}

	// disableIPLogging (etherpad-lite: disableIPlogging) is the old form of
	// ipLogging; it only counts when ipLogging itself is not configured.
	if viper.InConfig("disableiplogging") && !viper.InConfig("iplogging") && os.Getenv("ETHERPAD_IPLOGGING") == "" {
		if viper.GetBool("disableIPLogging") {
			viper.Set("ipLogging", IPLoggingAnonymous)
		} else {
			viper.Set("ipLogging", IPLoggingFull)
		}
	}

	// --- Unmarshal ---
	var s Settings
	if err := viper.Unmarshal(&s); err != nil {
		return nil, err
	}
	for _, opt := range []**string{&s.PadOptions.UserName, &s.PadOptions.UserColor} {
		if *opt != nil && **opt == "" {
			*opt = nil
		}
	}

	switch s.IPLogging {
	case IPLoggingFull, IPLoggingTruncated, IPLoggingAnonymous:
	default:
		// An unknown mode must not log addresses.
		s.IPLogging = IPLoggingAnonymous
	}
	return &s, nil
}
