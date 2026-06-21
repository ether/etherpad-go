package sheet

import "testing"

func TestCellIsEmpty(t *testing.T) {
	var empty Cell
	if !empty.IsEmpty() {
		t.Fatal("zero-value cell should be empty")
	}
	c := Cell{Raw: "42"}
	if c.IsEmpty() {
		t.Fatal("cell with raw should not be empty")
	}
	styled := Cell{StyleId: 3}
	if styled.IsEmpty() {
		t.Fatal("cell with style should not be empty")
	}
}

func TestCellRefComparable(t *testing.T) {
	a := CellRef{Row: 1, Col: 2}
	b := CellRef{Row: 1, Col: 2}
	if a != b {
		t.Fatal("CellRef values with equal coords must be equal (map key usable)")
	}
}

func TestCellKind(t *testing.T) {
	if (Cell{Raw: "=SUM(A1:A2)"}).Kind() != KindFormula {
		t.Fatal("leading = should be formula")
	}
	if (Cell{Raw: "42"}).Kind() != KindValue {
		t.Fatal("plain raw should be value")
	}
}
