package settings

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

func ReadConfig() (*Settings, error) {
	viper.Reset()
	ignoreServiceLinkPort()

	viper.SetEnvPrefix("etherpad")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	ApplyRegistryDefaults()

	viper.SetConfigType("json")
	// settings.json may hold comments, trailing commas and ${VAR:default}
	// values, like etherpad-lite's; viper reads plain JSON.
	file, saveFile, err := settingsFiles()
	if err != nil {
		return nil, err
	}
	if file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, fmt.Errorf("settings file %s: %w", file, err)
		}
		prepared, err := PrepareSettingsJSON(raw)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", file, err)
		}
		if err := viper.ReadConfig(bytes.NewReader(prepared)); err != nil {
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

	report, err := applyEnv(os.Environ())
	if err != nil {
		return nil, fmt.Errorf("settings from the environment: %w", err)
	}
	reportEnv(report)

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
	s.SettingsFile, s.SettingsSaveFile = file, saveFile
	return &s, nil
}

// ignoreServiceLinkPort drops ETHERPAD_PORT when Kubernetes set it, not the
// admin: for a Service named "etherpad" every pod in the namespace gets
// ETHERPAD_PORT=tcp://<cluster ip>:<port> (service links), which would be
// read as the port to listen on. A real port is a number.
func ignoreServiceLinkPort() {
	v := os.Getenv(EnvVar(Port))
	if strings.HasPrefix(v, "tcp://") || strings.HasPrefix(v, "udp://") || strings.HasPrefix(v, "sctp://") {
		fmt.Fprintf(os.Stderr, "ignoring %s=%s: set by Kubernetes for a Service named etherpad (service links), not a port; set enableServiceLinks: false in the pod spec to avoid it\n", EnvVar(Port), v)
		_ = os.Unsetenv(EnvVar(Port))
	}
}
