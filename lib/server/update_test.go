package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestUpdateChecker_CheckForUpdates(t *testing.T) {
	logger := zap.NewNop().Sugar()

	tests := []struct {
		name           string
		currentVersion string
		latestVersion  string
		wantUpdate     bool
		statusCode     int
	}{
		{
			name:           "Update available",
			currentVersion: "v1.0.0",
			latestVersion:  "v1.1.0",
			wantUpdate:     true,
			statusCode:     http.StatusOK,
		},
		{
			name:           "Already up to date",
			currentVersion: "v1.1.0",
			latestVersion:  "v1.1.0",
			wantUpdate:     false,
			statusCode:     http.StatusOK,
		},
		{
			name:           "GitHub API error",
			currentVersion: "v1.0.0",
			latestVersion:  "v1.1.0",
			wantUpdate:     false,
			statusCode:     http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.statusCode != http.StatusOK {
					w.WriteHeader(tt.statusCode)
					return
				}
				release := GitHubRelease{TagName: tt.latestVersion}
				json.NewEncoder(w).Encode(release)
			}))
			defer server.Close()

			uc := NewUpdateChecker(logger)
			uc.apiURL = server.URL

			updateAvailable, err := uc.CheckForUpdates(tt.currentVersion)
			if tt.statusCode != http.StatusOK {
				if err != nil {
					// Error is expected for non-200 status codes in some cases,
					// or updateAvailable should be nil/false.
					// Current implementation returns nil, nil for non-200.
					if updateAvailable != nil {
						t.Errorf("expected nil updateAvailable for status %d, got %v", tt.statusCode, *updateAvailable)
					}
					return
				}
				if updateAvailable != nil && *updateAvailable {
					t.Errorf("expected no update available for status %d", tt.statusCode)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if updateAvailable == nil {
				t.Fatalf("expected updateAvailable to be not nil")
			}

			if *updateAvailable != tt.wantUpdate {
				t.Errorf("got updateAvailable = %v, want %v", *updateAvailable, tt.wantUpdate)
			}
		})
	}
}
