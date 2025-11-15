package utils

import (
	"errors"
	"strconv"

	"github.com/ether/etherpad-go/lib/db"
	plugins2 "github.com/ether/etherpad-go/lib/plugins"
	"github.com/ether/etherpad-go/lib/settings"
	"go.uber.org/zap"
)

func init() {
	GetPlugins()
}

var ColorPalette = []string{
	"#ffc7c7",
	"#fff1c7",
	"#e3ffc7",
	"#c7ffd5",
	"#c7ffff",
	"#c7d5ff",
	"#e3c7ff",
	"#ffc7f1",
	"#ffa8a8",
	"#ffe699",
	"#cfff9e",
	"#99ffb3",
	"#a3ffff",
	"#99b3ff",
	"#cc99ff",
	"#ff99e5",
	"#e7b1b1",
	"#e9dcAf",
	"#cde9af",
	"#bfedcc",
	"#b1e7e7",
	"#c3cdee",
	"#d2b8ea",
	"#eec3e6",
	"#e9cece",
	"#e7e0ca",
	"#d3e5c7",
	"#bce1c5",
	"#c1e2e2",
	"#c1c9e2",
	"#cfc1e2",
	"#e0bdd9",
	"#baded3",
	"#a0f8eb",
	"#b1e7e0",
	"#c3c8e4",
	"#cec5e2",
	"#b1d5e7",
	"#cda8f0",
	"#f0f0a8",
	"#f2f2a6",
	"#f5a8eb",
	"#c5f9a9",
	"#ececbb",
	"#e7c4bc",
	"#daf0b2",
	"#b0a0fd",
	"#bce2e7",
	"#cce2bb",
	"#ec9afe",
	"#edabbd",
	"#aeaeea",
	"#c4e7b1",
	"#d722bb",
	"#f3a5e7",
	"#ffa8a8",
	"#d8c0c5",
	"#eaaedd",
	"#adc6eb",
	"#bedad1",
	"#dee9af",
	"#e9afc2",
	"#f8d2a0",
	"#b3b3e6",
}

func GetDB(retrievedSettings settings.Settings, setupLogger *zap.SugaredLogger) (db.DataStore, error) {
	if retrievedSettings.DBType == settings.SQLITE {
		setupLogger.Infof("Using SQLite database at %s", retrievedSettings.DBSettings.Filename)
		return db.NewSQLiteDB(retrievedSettings.DBSettings.Filename)
	} else if retrievedSettings.DBType == settings.MEMORY {
		setupLogger.Info("Using in-memory database (data will be lost on restart)")
		return db.NewMemoryDataStore(), nil
	} else if retrievedSettings.DBType == settings.POSTGRES {
		setupLogger.Infof("Using Postgres database at %s with database %s", retrievedSettings.DBSettings.Host, retrievedSettings.DBSettings.Database)

		port, err := strconv.Atoi(retrievedSettings.DBSettings.Port)
		if err != nil {
			return nil, err
		}

		return db.NewPostgresDB(db.PostgresOptions{
			Username: retrievedSettings.DBSettings.User,
			Password: retrievedSettings.DBSettings.Password,
			Host:     retrievedSettings.DBSettings.Host,
			Database: retrievedSettings.DBSettings.Database,
			Port:     port,
		})
	}
	return nil, errors.New("unsupported database type")
}

var plugins = map[string]plugins2.Plugin{}
var parts = map[string]plugins2.Part{}
var packages = map[string]plugins2.Plugin{}

func GetPlugins() map[string]plugins2.Plugin {
	if len(plugins) == 0 {
		packages, parts, plugins = plugins2.Update()
	}
	return plugins
}

func GetParts() map[string]plugins2.Part {
	if parts == nil {
		packages, parts, plugins = plugins2.Update()
	}
	return parts
}

func GetPackages() map[string]plugins2.Plugin {
	if packages == nil {
		packages, parts, plugins = plugins2.Update()
	}
	return packages
}
