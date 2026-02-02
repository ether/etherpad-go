package server

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
	"go.uber.org/zap"
)

type UpdateChecker struct {
	httpClient *http.Client
	logger     *zap.SugaredLogger
	db         db.DataStore
	apiURL     string
}

func NewUpdateChecker(logger *zap.SugaredLogger, db db.DataStore) *UpdateChecker {
	return &UpdateChecker{
		httpClient: &http.Client{},
		logger:     logger,
		db:         db,
		apiURL:     "https://api.github.com/repos/ether/etherpad-go/releases/latest",
	}
}

type GitHubRelease struct {
	TagName string `json:"tag_name"`
}

func StartUpdateRoutine(logger *zap.SugaredLogger, db db.DataStore, currentVersion string) {
	uc := NewUpdateChecker(logger, db)
	go func() {
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// Initial check
		uc.performUpdateCheck(currentVersion)

		for range ticker.C {
			uc.performUpdateCheck(currentVersion)
		}
	}()
}

func (uc *UpdateChecker) performUpdateCheck(currentVersion string) {
	updateAvailable, err := uc.CheckForUpdates(currentVersion)
	if err != nil {
		uc.logger.Warnf("Failed to check for updates: %v", err)
		return
	}

	if updateAvailable != nil && *updateAvailable {
		uc.logger.Info("A new version of Etherpad Go is available! Please update to the latest version.")
	}

	if err := uc.db.SaveServerVersion(currentVersion); err != nil {
		uc.logger.Warnf("Failed to persist current version to database: %v", err)
	}
}

func (uc *UpdateChecker) CheckForUpdates(currentVersion string) (*bool, error) {
	resp, err := uc.httpClient.Get(uc.apiURL)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, nil
	}

	var release GitHubRelease
	err = json.NewDecoder(resp.Body).Decode(&release)
	if err != nil {
		return nil, err
	}

	update := utils.IsUpdateAvailable(currentVersion, release.TagName)
	return &update, nil
}
