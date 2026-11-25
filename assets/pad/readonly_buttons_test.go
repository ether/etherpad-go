package pad

import (
	"bytes"
	"context"
	"strings"
	"testing"

	pluginTypes "github.com/ether/etherpad-go/lib/models/plugins"
)

// Plugin buttons that do not change the pad (readOnly in ep.json, e.g.
// print) are marked, so read-only pads keep them while the editing buttons
// are hidden.
func TestPluginButtonsMarkedForReadOnlyPads(t *testing.T) {
	groups := []pluginTypes.ToolbarButtonGroup{{
		PluginName: "ep_test",
		Buttons: []pluginTypes.ToolbarButton{
			{Key: "print", Title: "print", Type: "button", ReadOnly: true},
			{Key: "align", Title: "align", Type: "button"},
		},
	}}
	var buf bytes.Buffer
	if err := PluginToolbarButtons(map[string]string{}, groups).Render(context.Background(), &buf); err != nil {
		t.Fatal(err)
	}
	html := buf.String()
	li := func(key string) string {
		i := strings.Index(html, `data-key="`+key+`"`)
		if i < 0 {
			t.Fatalf("no button %s in %s", key, html)
		}
		j := strings.Index(html[i:], ">")
		return html[i : i+j]
	}
	if !strings.Contains(li("print"), "data-readonly-ok") {
		t.Errorf("print is not marked: %s", li("print"))
	}
	if strings.Contains(li("align"), "data-readonly-ok") {
		t.Errorf("align is marked: %s", li("align"))
	}
}
