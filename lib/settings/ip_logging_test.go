package settings

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Port of etherpad-lite's ipLoggingSetting spec.
func TestIPLoggingSetting(t *testing.T) {
	read := func(t *testing.T, json string) *Settings {
		t.Helper()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.json"), []byte(json), 0o600))
		t.Chdir(dir)
		cfg, err := ReadConfig()
		require.NoError(t, err)
		return cfg
	}
	t.Run("the mode is honoured", func(t *testing.T) {
		assert.Equal(t, "ANONYMOUS", read(t, `{"ipLogging":"anonymous"}`).LogIP("1.2.3.4"))
		assert.Equal(t, "1.2.3.4", read(t, `{"ipLogging":"full"}`).LogIP("1.2.3.4"))
		assert.Equal(t, "1.2.3.0", read(t, `{"ipLogging":"truncated"}`).LogIP("1.2.3.4"))
		assert.Equal(t, "2001:db8:1::", read(t, `{"ipLogging":"truncated"}`).LogIP("2001:db8:1:2:3:4:5:6"))
	})
	t.Run("disableIPLogging is mapped", func(t *testing.T) {
		assert.Equal(t, IPLoggingAnonymous, read(t, `{"disableIPlogging":true}`).IPLogging)
		assert.Equal(t, IPLoggingFull, read(t, `{"disableIPLogging":false}`).IPLogging)
		assert.Equal(t, IPLoggingTruncated, read(t, `{"disableIPLogging":true,"ipLogging":"truncated"}`).IPLogging, "ipLogging wins")
		assert.Equal(t, IPLoggingAnonymous, read(t, `{}`).IPLogging, "default")
	})
	t.Run("an invalid mode falls back to anonymous", func(t *testing.T) {
		cfg := read(t, `{"ipLogging":"lolnope"}`)
		assert.Equal(t, IPLoggingAnonymous, cfg.IPLogging)
		assert.Equal(t, "ANONYMOUS", cfg.LogIP("8.8.8.8"))
		assert.Equal(t, IPLoggingAnonymous, read(t, `{"ipLogging":null}`).IPLogging)
	})
}

// Every log line that mentions a client IP goes through LogIP.
func TestNoRawIPInLogs(t *testing.T) {
	root := filepath.Join("..", "..")
	logCall := regexp.MustCompile(`\.(Info|Warn|Error|Debug)(f|w)?\(|log\.Print`)
	rawIP := regexp.MustCompile(`\.ClientIP\b|\.IP\(\)`)
	var offending []string
	err := filepath.Walk(filepath.Join(root, "lib"), func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(content), "\n") {
			if logCall.MatchString(line) && rawIP.MatchString(line) && !strings.Contains(line, "LogIP(") {
				offending = append(offending, path+":"+strconv.Itoa(i+1)+": "+strings.TrimSpace(line))
			}
		}
		return nil
	})
	require.NoError(t, err)
	assert.Empty(t, offending, "log lines with a raw client IP")
}
