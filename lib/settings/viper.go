package settings

import (
	"errors"
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

	// --- Unmarshal ---
	var s Settings
	if err := viper.Unmarshal(&s); err != nil {
		return nil, err
	}

	return &s, nil
}
