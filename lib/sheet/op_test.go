package sheet

import (
	"encoding/json"
	"testing"
)

func TestOpJSONRoundTrip(t *testing.T) {
	raw := "=SUM(A1:A2)"
	ops := []Op{
		{Type: OpSetCell, Sheet: "s1", Row: 2, Col: 3, Raw: &raw},
		{Type: OpInsertRows, Sheet: "s1", Index: 5, Count: 2},
		{Type: OpClearRange, Sheet: "s1", Row: 0, Col: 0, EndRow: 3, EndCol: 3},
	}
	for _, op := range ops {
		b, err := json.Marshal(op)
		if err != nil {
			t.Fatalf("marshal: %v", err)
		}
		var got Op
		if err := json.Unmarshal(b, &got); err != nil {
			t.Fatalf("unmarshal: %v", err)
		}
		if got.Type != op.Type || got.Sheet != op.Sheet {
			t.Fatalf("round-trip mismatch: %+v vs %+v", got, op)
		}
	}
}

func TestOpValidate(t *testing.T) {
	if (Op{Type: OpInsertRows, Sheet: "s1", Index: 0, Count: 0}).Validate() == nil {
		t.Fatal("insertRows with count 0 must be invalid")
	}
	if (Op{Type: OpInsertRows, Sheet: "s1", Index: -1, Count: 1}).Validate() == nil {
		t.Fatal("negative index must be invalid")
	}
	raw := "x"
	if err := (Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 0, Raw: &raw}).Validate(); err != nil {
		t.Fatalf("valid setCell rejected: %v", err)
	}
	if (Op{Type: "bogus", Sheet: "s1"}).Validate() == nil {
		t.Fatal("unknown op type must be invalid")
	}
	if (Op{Type: OpSetCell, Sheet: "", Row: 0, Col: 0, Raw: &raw}).Validate() == nil {
		t.Fatal("missing sheet must be invalid")
	}
}

func TestOpValidateProps(t *testing.T) {
	ok := map[string]string{
		"bold": "1", "italic": "1", "underline": "1",
		"color": "#c00", "bg": "#ffcc00", "align": "center",
		"border": "all", "numFmt": "currency:2",
		"fontFamily": "Times New Roman", "fontSize": "96", "wrap": "1",
	}
	if err := (Op{Type: OpSetStyle, Sheet: "s1", Props: ok}).Validate(); err != nil {
		t.Fatalf("valid props rejected: %v", err)
	}
	bad := []map[string]string{
		{"bg": "url(https://evil.example/x)"},  // CSS injection
		{"color": "red"},                       // not hex
		{"bold": "yes"},                        // not "1"
		{"align": "justify"},                   // outside vocabulary
		{"numFmt": "number:999"},               // decimals capped at 2 digits
		{"expression": "alert(1)"},             // unknown key
		{"border": "1px solid url(https://x)"}, // only "all"
		{"fontFamily": "Comic Sans MS"},        // outside allowlist
		{"fontSize": "0"},                      // below range
		{"fontSize": "97"},                     // above range
		{"fontSize": "012"},                    // leading zero
		{"fontSize": "12.5"},                   // not an integer
		{"wrap": "yes"},                        // only "1"
	}
	for _, props := range bad {
		if (Op{Type: OpSetStyle, Sheet: "s1", Props: props}).Validate() == nil {
			t.Fatalf("props %v must be invalid", props)
		}
		if (Op{Type: OpSetCell, Sheet: "s1", Props: props}).Validate() == nil {
			t.Fatalf("setCell props %v must be invalid", props)
		}
	}
}
