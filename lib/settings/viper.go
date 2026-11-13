package settings

import (
	"errors"
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

	viper.SetConfigName("settings")
	viper.SetConfigType("json")
	viper.AddConfigPath(".")

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, err
		}
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
