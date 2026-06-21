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
