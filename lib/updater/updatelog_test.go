package updater

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

// Port of etherpad-lite's updateLog spec.

func TestTailLines(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "update.log")
	assert.Equal(t, []string{}, TailLines(p, 5), "missing file")
	require.NoError(t, os.WriteFile(p, nil, 0o600))
	assert.Equal(t, []string{}, TailLines(p, 5), "empty file")

	require.NoError(t, os.WriteFile(p, []byte("a\nb\nc\n"), 0o600))
	assert.Equal(t, []string{"a", "b", "c"}, TailLines(p, 5), "shorter than n")
	var many strings.Builder
	for i := 1; i <= 10; i++ {
		many.WriteString(strings.Repeat("x", i) + "\n")
	}
	require.NoError(t, os.WriteFile(p, []byte(many.String()), 0o600))
	assert.Equal(t, []string{"xxxxxxxx", "xxxxxxxxx", "xxxxxxxxxx"}, TailLines(p, 3), "the last n")

	require.NoError(t, os.WriteFile(p, []byte("a\r\nb\nlast"), 0o600))
	assert.Equal(t, []string{"a", "b", "last"}, TailLines(p, 5), "a last line without newline, CRLF from Windows")
	assert.Equal(t, []string{}, TailLines(p, 0))
}

func TestAppendLine(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "a", "b", "update.log")
	AppendLine(nested, "hello world")
	b, err := os.ReadFile(nested)
	require.NoError(t, err)
	assert.Equal(t, "hello world\n", string(b))

	collide := filepath.Join(dir, "not-a-dir")
	require.NoError(t, os.WriteFile(collide, []byte("oops"), 0o600))
	AppendLine(filepath.Join(collide, "inner", "update.log"), "x") // must not panic
}

func TestRotateIfNeeded(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "update.log")
	read := func(name string) string {
		b, err := os.ReadFile(name)
		require.NoError(t, err, name)
		return string(b)
	}

	require.NoError(t, os.WriteFile(p, []byte(strings.Repeat("a", 50)), 0o600))
	RotateIfNeeded(p, 10, 3)
	assert.Equal(t, strings.Repeat("a", 50), read(p+".1"))
	assert.NoFileExists(t, p)

	require.NoError(t, os.WriteFile(p, []byte(strings.Repeat("newest", 20)), 0o600))
	require.NoError(t, os.WriteFile(p+".1", []byte("older-1"), 0o600))
	require.NoError(t, os.WriteFile(p+".2", []byte("older-2"), 0o600))
	require.NoError(t, os.WriteFile(p+".3", []byte("oldest"), 0o600))
	RotateIfNeeded(p, 10, 3)
	assert.Equal(t, strings.Repeat("newest", 20), read(p+".1"))
	assert.Equal(t, "older-1", read(p+".2"))
	assert.Equal(t, "older-2", read(p+".3"), "the oldest is dropped")
	assert.NoFileExists(t, p+".4")

	require.NoError(t, os.WriteFile(p, []byte("small"), 0o600))
	_ = os.Remove(p + ".1")
	RotateIfNeeded(p, 10<<20, 3)
	assert.Equal(t, "small", read(p))
	assert.NoFileExists(t, p+".1")
}

// The updater's messages land in the update log, and LogTail returns them.
func TestUpdaterWritesUpdateLog(t *testing.T) {
	p := filepath.Join(t.TempDir(), "var", "update.log")
	logger := withUpdateLog(zap.NewNop().Sugar(), p)
	logger.Infof("updater: scheduled auto-apply of %s", "v2.0.0")
	logger.Debug("updater: not in the log")
	logger.Warn("updater: drain cancelled")

	u := &Updater{cfg: Config{LogPath: p}}
	lines := u.LogTail(10)
	require.Len(t, lines, 2)
	assert.Contains(t, lines[0], "INFO")
	assert.Contains(t, lines[0], "updater: scheduled auto-apply of v2.0.0")
	assert.Contains(t, lines[1], "WARN")
	assert.Equal(t, []string{}, (&Updater{}).LogTail(10), "no log configured")
}
