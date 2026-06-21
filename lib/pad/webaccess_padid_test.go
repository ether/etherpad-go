package pad

import "testing"

func TestExtractEncodedPadId(t *testing.T) {
	cases := map[string]string{
		"/p/foo":             "foo",
		"/s/foo":             "foo", // spreadsheet pages share the pad auth model
		"/s/foo/import":      "foo",
		"/s/foo/export.xlsx": "foo",
		"/p/foo/timeslider":  "foo",
		"/p/a%20b":           "a%20b", // still encoded; caller unescapes
		"/other":             "",
		"/":                  "",
		"/s/":                "",
		"/p/":                "",
	}
	for path, want := range cases {
		if got := extractEncodedPadId(path); got != want {
			t.Errorf("extractEncodedPadId(%q) = %q; want %q", path, got, want)
		}
	}
}
