package server

import (
	"encoding/json"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type UpdateChecker struct {
	httpClient *http.Client
	logger     *zap.SugaredLogger
	apiURL     string
}

func NewUpdateChecker(logger *zap.SugaredLogger) *UpdateChecker {
	return &UpdateChecker{
		httpClient: &http.Client{},
		logger:     logger,
		apiURL:     "https://api.github.com/repos/ether/etherpad-go/releases/latest",
	}
}

type GitHubRelease struct {
	TagName string `json:"tag_name"`
}

func StartUpdateRoutine(logger *zap.SugaredLogger, currentVersion string) {
	uc := NewUpdateChecker(logger)
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

	if release.TagName != currentVersion {
		updateAvailable := true
		return &updateAvailable, nil
	}
	updateAvailable := false
	return &updateAvailable, nil
}
