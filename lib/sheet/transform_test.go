package sheet

import "testing"

func TestTransformCellAgainstInsertRows(t *testing.T) {
	applied := Op{Type: OpInsertRows, Sheet: "s1", Index: 2, Count: 3}
	in := Op{Type: OpSetCell, Sheet: "s1", Row: 4, Col: 0, Raw: ptr("x")}
	out := Transform(in, applied)
	if out.Row != 7 {
		t.Fatalf("cell below insert must shift down by 3: got row %d", out.Row)
	}
	above := Transform(Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 0, Raw: ptr("y")}, applied)
	if above.Row != 1 {
		t.Fatalf("cell above insert must not move: got row %d", above.Row)
	}
}

func TestTransformCellAgainstDeleteRows(t *testing.T) {
	applied := Op{Type: OpDeleteRows, Sheet: "s1", Index: 2, Count: 2} // deletes rows 2,3
	below := Transform(Op{Type: OpSetCell, Sheet: "s1", Row: 5, Col: 0, Raw: ptr("x")}, applied)
	if below.Row != 3 {
		t.Fatalf("cell below delete must shift up by 2: got %d", below.Row)
	}
	// A cell inside the deleted band: clamp to the deletion index (its row is gone).
	inside := Transform(Op{Type: OpSetCell, Sheet: "s1", Row: 3, Col: 0, Raw: ptr("z")}, applied)
	if inside.Row != 2 {
		t.Fatalf("cell inside deleted band should clamp to index 2: got %d", inside.Row)
	}
}

func TestTransformDifferentSheetIsNoop(t *testing.T) {
	applied := Op{Type: OpInsertRows, Sheet: "other", Index: 0, Count: 5}
	in := Op{Type: OpSetCell, Sheet: "s1", Row: 1, Col: 1, Raw: ptr("x")}
	out := Transform(in, applied)
	if out.Row != 1 || out.Col != 1 {
		t.Fatal("ops on different sheets must not transform")
	}
}

func TestTransformInsertAgainstInsert(t *testing.T) {
	applied := Op{Type: OpInsertRows, Sheet: "s1", Index: 2, Count: 2}
	in := Op{Type: OpInsertRows, Sheet: "s1", Index: 4, Count: 1}
	out := Transform(in, applied)
	if out.Index != 6 {
		t.Fatalf("later insert index must shift by applied count: got %d", out.Index)
	}
}

func TestTransformColsAgainstInsertCols(t *testing.T) {
	applied := Op{Type: OpInsertCols, Sheet: "s1", Index: 1, Count: 2}
	in := Op{Type: OpSetCell, Sheet: "s1", Row: 0, Col: 3, Raw: ptr("x")}
	out := Transform(in, applied)
	if out.Col != 5 {
		t.Fatalf("cell right of col insert must shift by 2: got %d", out.Col)
	}
}
