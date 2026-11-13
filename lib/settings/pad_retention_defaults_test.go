package settings

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// etherpad-lite padRetention "defaults to dry run": turning the feature on
// with only a deletion age must not delete anything yet, and group pads
// stay excluded.
func TestPadRetentionDefaultsToDryRun(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "settings.json"),
		[]byte(`{"padRetention": {"enabled": true, "deleteAfterDays": 30}}`), 0o600))
	t.Chdir(dir)
	cfg, err := ReadConfig()
	require.NoError(t, err)
	require.True(t, cfg.PadRetention.Enabled)
	require.Equal(t, 30, cfg.PadRetention.DeleteAfterDays)
	require.True(t, cfg.PadRetention.DryRun)
	require.True(t, cfg.PadRetention.ExcludeGroupPads)
	require.Equal(t, 1, cfg.PadRetention.SweepIntervalHours)
}
