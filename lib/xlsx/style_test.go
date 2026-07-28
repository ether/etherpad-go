package xlsx

import "testing"

// The two mappings must be inverses for everything the prop allowlist accepts,
// otherwise an Excel round trip silently drops formatting.
func TestStylePropsRoundTrip(t *testing.T) {
	props := map[string]string{
		"bold": "1", "italic": "1", "underline": "1", "strike": "1",
		"align": "center", "valign": "middle", "wrap": "1",
		"color": "#cc0000", "bg": "#ffcc00", "fontFamily": "Arial", "fontSize": "14",
	}
	got := styleToProps(propsToStyle(props))
	for k, want := range props {
		if got[k] != want {
			t.Errorf("prop %q: got %q, want %q", k, got[k], want)
		}
	}
}

func TestVerticalAlignmentNaming(t *testing.T) {
	// xlsx calls the middle alignment "center"; the model calls it "middle".
	if v := propsToStyle(map[string]string{"valign": "middle"}).Alignment.Vertical; v != "center" {
		t.Errorf("valign middle exported as %q, want center", v)
	}
	for _, v := range []string{"top", "bottom"} {
		if got := propsToStyle(map[string]string{"valign": v}).Alignment.Vertical; got != v {
			t.Errorf("valign %q exported as %q", v, got)
		}
	}
}
