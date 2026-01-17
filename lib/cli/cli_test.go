package cli

import (
	"testing"
)

func TestParseCLIArgs(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantHost   string
		wantAppend string
	}{
		{
			name:       "no arguments",
			args:       []string{},
			wantHost:   "",
			wantAppend: "",
		},
		{
			name:       "positional host",
			args:       []string{"http://test.com"},
			wantHost:   "http://test.com",
			wantAppend: "",
		},
		{
			name:       "explicit flags",
			args:       []string{"-host", "http://test.com", "-append", "hello"},
			wantHost:   "http://test.com",
			wantAppend: "hello",
		},
		{
			name:       "shorthand append",
			args:       []string{"http://test.com", "-a", "world"},
			wantHost:   "http://test.com",
			wantAppend: "world",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host, appendStr, err := parseCLIArgs(tt.args)
			if err != nil {
				t.Errorf("parseCLIArgs() error = %v", err)
				return
			}
			if host != tt.wantHost {
				t.Errorf("host = %v, want %v", host, tt.wantHost)
			}
			if appendStr != tt.wantAppend {
				t.Errorf("appendStr = %v, want %v", appendStr, tt.wantAppend)
			}
		})
	}
}
