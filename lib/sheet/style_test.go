package sheet

import "testing"

func TestStylePoolDedup(t *testing.T) {
	p := NewStylePool()
	id1 := p.Put(Style{Props: map[string]string{"bold": "1", "numFmt": "0.00"}})
	id2 := p.Put(Style{Props: map[string]string{"numFmt": "0.00", "bold": "1"}}) // same, different order
	if id1 != id2 {
		t.Fatalf("equal styles must dedup to same id, got %d and %d", id1, id2)
	}
	id3 := p.Put(Style{Props: map[string]string{"bold": "1"}})
	if id3 == id1 {
		t.Fatal("different styles must get different ids")
	}
}

func TestStylePoolPutCopiesProps(t *testing.T) {
	p := NewStylePool()
	props := map[string]string{"bold": "1"}
	id := p.Put(Style{Props: props})
	props["bold"] = "0" // caller mutates its map after interning
	got, _ := p.Get(id)
	if got.Props["bold"] != "1" {
		t.Fatal("Put must copy props; pooled style was mutated through the caller's map")
	}
	if again := p.Put(Style{Props: map[string]string{"bold": "1"}}); again != id {
		t.Fatalf("dedup broken after caller mutation: got %d, want %d", again, id)
	}
}

func TestStylePoolEmptyIsZero(t *testing.T) {
	p := NewStylePool()
	if got := p.Put(Style{}); got != 0 {
		t.Fatalf("empty style must map to id 0, got %d", got)
	}
	if got := p.Put(Style{Props: map[string]string{}}); got != 0 {
		t.Fatalf("style with empty props must map to id 0, got %d", got)
	}
}

func TestStylePoolGet(t *testing.T) {
	p := NewStylePool()
	id := p.Put(Style{Props: map[string]string{"color": "#ff0000"}})
	s, ok := p.Get(id)
	if !ok || s.Props["color"] != "#ff0000" {
		t.Fatalf("Get(%d) failed: ok=%v style=%+v", id, ok, s)
	}
}
