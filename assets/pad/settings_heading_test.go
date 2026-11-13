package pad

import (
	"bytes"
	"context"
	"testing"
)

// Port of etherpad-lite's settingsModalHeading: the settings dialog is
// titled "Settings" whether or not pad-wide settings are on; the pad-wide
// section, with its own heading, exists only when they are.
func TestSettingsPopupHeading(t *testing.T) {
	tr := map[string]string{"pad.settings.title": "Settings", "pad.settings.padSettings": "Pad-wide Settings"}
	for _, padWide := range []bool{true, false} {
		var buf bytes.Buffer
		if err := SettingsPopup(tr, nil, nil, padWide).Render(context.Background(), &buf); err != nil {
			t.Fatal(err)
		}
		html := buf.String()
		if !bytes.Contains(buf.Bytes(), []byte(`<h1 id="padSettingsTitle">Settings</h1>`)) {
			t.Errorf("padWideSettings=%v: title is not Settings: %s", padWide, html)
		}
		hasSection := bytes.Contains(buf.Bytes(), []byte(`id="pad-settings-section"`))
		if hasSection != padWide {
			t.Errorf("padWideSettings=%v: pad-wide section present=%v", padWide, hasSection)
		}
		if !padWide && bytes.Contains(buf.Bytes(), []byte("Pad-wide Settings")) {
			t.Errorf("no misleading Pad-wide label without pad-wide settings")
		}
	}
}
