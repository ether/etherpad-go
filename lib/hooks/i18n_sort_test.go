package hooks

import "testing"

// Port of etherpad-lite's i18n spec: languages are sorted by their native
// name, without regard to case or accents.
func TestSortAvailableLangs(t *testing.T) {
	langs := map[string]LanguageInfo{
		"de": {DisplayName: "Deutsch"},
		"cs": {DisplayName: "Čeština"},
		"en": {DisplayName: "English"},
		"es": {DisplayName: "Español"},
		"is": {DisplayName: "Íslenska"},
		"fr": {DisplayName: "français"},
		"el": {DisplayName: "Ελληνικά"},
	}
	got := sortAvailableLangs(langs)
	want := []string{"cs", "de", "en", "es", "fr", "is", "el"}
	if len(got) != len(want) {
		t.Fatalf("got %d entries", len(got))
	}
	for i, code := range want {
		if got[i].Code != code {
			names := []string{}
			for _, e := range got {
				names = append(names, e.Info.DisplayName)
			}
			t.Fatalf("order %v, want codes %v", names, want)
		}
	}
}
