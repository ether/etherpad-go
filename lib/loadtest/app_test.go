package loadtest

import (
	"testing"
)

func TestParseRunArgs(t *testing.T) {
	tests := []struct {
		name          string
		args          []string
		wantHost      string
		wantAuthors   int
		wantLurkers   int
		wantDuration  int
		wantUntilFail bool
	}{
		{
			name:         "default values",
			args:         []string{},
			wantHost:     "http://127.0.0.1:9001",
			wantAuthors:  0,
			wantLurkers:  0,
			wantDuration: 0,
		},
		{
			name:         "positional host",
			args:         []string{"http://test.com"},
			wantHost:     "http://test.com",
			wantAuthors:  0,
			wantLurkers:  0,
			wantDuration: 0,
		},
		{
			name:          "explicit flags",
			args:          []string{"-host", "http://test.com", "-authors", "5", "-lurkers", "10", "-duration", "60", "-loadUntilFail"},
			wantHost:      "http://test.com",
			wantAuthors:   5,
			wantLurkers:   10,
			wantDuration:  60,
			wantUntilFail: true,
		},
		{
			name:         "positional host and flags",
			args:         []string{"http://pos.com", "-authors", "3"},
			wantHost:     "http://pos.com",
			wantAuthors:  3,
			wantLurkers:  0,
			wantDuration: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, authors, lurkers, duration, untilFail, err := parseRunArgs(tt.args)
			if err != nil {
				t.Errorf("parseRunArgs() error = %v", err)
				return
			}
			if host != tt.wantHost {
				t.Errorf("host = %v, want %v", host, tt.wantHost)
			}
			if authors != tt.wantAuthors {
				t.Errorf("authors = %v, want %v", authors, tt.wantAuthors)
			}
			if lurkers != tt.wantLurkers {
				t.Errorf("lurkers = %v, want %v", lurkers, tt.wantLurkers)
			}
			if duration != tt.wantDuration {
				t.Errorf("duration = %v, want %v", duration, tt.wantDuration)
			}
			if untilFail != tt.wantUntilFail {
				t.Errorf("untilFail = %v, want %v", untilFail, tt.wantUntilFail)
			}
		})
	}
}

func TestParseMultiRunArgs(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantHost    string
		wantMaxPads int
	}{
		{
			name:        "default values",
			args:        []string{},
			wantHost:    "http://127.0.0.1:9001",
			wantMaxPads: 10,
		},
		{
			name:        "explicit flags",
			args:        []string{"-host", "http://test.com", "-maxPads", "20"},
			wantHost:    "http://test.com",
			wantMaxPads: 20,
		},
		{
			name:        "positional host",
			args:        []string{"http://pos.com", "-maxPads", "5"},
			wantHost:    "http://pos.com",
			wantMaxPads: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, maxPads, err := parseMultiRunArgs(tt.args)
			if err != nil {
				t.Errorf("parseMultiRunArgs() error = %v", err)
				return
			}
			if host != tt.wantHost {
				t.Errorf("host = %v, want %v", host, tt.wantHost)
			}
			if maxPads != tt.wantMaxPads {
				t.Errorf("maxPads = %v, want %v", maxPads, tt.wantMaxPads)
			}
		})
	}
}
