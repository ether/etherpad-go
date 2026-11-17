package sheet

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"strings"
	"unicode"
)

// Props that are not formatting but belong to the cell, like Excel's
// hyperlinks, notes, data validation and conditional formatting. They live in
// the style props so they move, copy and undo with the cell.
const (
	PropLink      = "link"      // http(s) or mailto URL
	PropNote      = "note"      // note text
	PropValidList = "validList" // allowed values, comma separated (Excel's list validation)
	PropCondFmt   = "cf"        // JSON array of CondRule
)

// MetaProps are kept by "Clear Formats" (Excel keeps notes, links and
// validation; it removes conditional formatting).
var MetaProps = []string{PropLink, PropNote, PropValidList}

const (
	// Limits count UTF-8 bytes, on the server (len) and in the browser
	// (TextEncoder), so both accept the same values.
	MaxLinkLength      = 2048
	MaxNoteLength      = 4000
	MaxValidListLength = 255 // Excel's limit for a list typed into the dialog
	MaxCondRules       = 8
	MaxCondValue       = 255
	// MaxCondJSON fits eight rules with two values of MaxCondValue bytes each,
	// even when JSON escapes every character of them.
	MaxCondJSON = 16384
)

// CondRule is one conditional-formatting rule of a cell. Op compares the
// cell's value with V1 (and V2 for between/notBetween); when it matches, the
// cell is shown with Bg, Color and Bold.
type CondRule struct {
	Op    string `json:"op"`
	V1    string `json:"v1,omitempty"`
	V2    string `json:"v2,omitempty"`
	Bg    string `json:"bg,omitempty"`
	Color string `json:"color,omitempty"`
	Bold  string `json:"bold,omitempty"`
}

// CondOps lists the rule operators and how many values they take.
var CondOps = map[string]int{
	"gt": 1, "lt": 1, "ge": 1, "le": 1, "eq": 1, "ne": 1,
	"between": 2, "notBetween": 2,
	"contains": 1, "notContains": 1, "beginsWith": 1, "endsWith": 1,
	"blank": 0, "notBlank": 0,
}

func validLink(v string) bool {
	if v == "" || len(v) > MaxLinkLength {
		return false
	}
	for _, c := range v {
		if unicode.IsSpace(c) || unicode.IsControl(c) {
			return false
		}
	}
	u, err := url.Parse(v)
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https":
		return u.Host != ""
	case "mailto":
		return u.Opaque != ""
	}
	return false
}

// ParseCondRules decodes a cf prop; nil when it is invalid.
func ParseCondRules(v string) []CondRule {
	if v == "" || len(v) > MaxCondJSON {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader([]byte(v)))
	dec.DisallowUnknownFields()
	var rules []CondRule
	if err := dec.Decode(&rules); err != nil {
		return nil
	}
	// Nothing may follow the array: the browser's JSON.parse would refuse it.
	if _, err := dec.Token(); err != io.EOF {
		return nil
	}
	if len(rules) == 0 || len(rules) > MaxCondRules {
		return nil
	}
	for _, r := range rules {
		n, ok := CondOps[r.Op]
		if !ok || len(r.V1) > MaxCondValue || len(r.V2) > MaxCondValue {
			return nil
		}
		if (n >= 1 && r.V1 == "") || (n == 2 && r.V2 == "") || (n < 2 && r.V2 != "") || (n == 0 && r.V1 != "") {
			return nil
		}
		if r.Bg == "" && r.Color == "" && r.Bold == "" {
			return nil // a rule that changes nothing
		}
		if (r.Bg != "" && !hexColorRe.MatchString(r.Bg)) || (r.Color != "" && !hexColorRe.MatchString(r.Color)) || (r.Bold != "" && r.Bold != "1") {
			return nil
		}
	}
	return rules
}

func validCondFmt(v string) bool { return ParseCondRules(v) != nil }
