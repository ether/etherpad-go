package sheet

import "testing"

// lcg is a tiny deterministic RNG so trials are reproducible across runs.
type lcg struct{ state uint64 }

func (r *lcg) next() uint64 {
	r.state = r.state*6364136223846793005 + 1442695040888963407
	return r.state >> 16
}

func (r *lcg) intn(n int) int {
	if n <= 0 {
		return 0
	}
	return int(r.next() % uint64(n))
}

func randomOp(r *lcg, baseRev int) Op {
	switch r.intn(5) {
	case 0:
		raw := "v"
		return Op{Type: OpSetCell, Sheet: "s1", Row: r.intn(8), Col: r.intn(8), Raw: &raw, BaseRev: baseRev}
	case 1:
		return Op{Type: OpInsertRows, Sheet: "s1", Index: r.intn(8), Count: 1 + r.intn(2), BaseRev: baseRev}
	case 2:
		return Op{Type: OpDeleteRows, Sheet: "s1", Index: r.intn(8), Count: 1 + r.intn(2), BaseRev: baseRev}
	case 3:
		return Op{Type: OpInsertCols, Sheet: "s1", Index: r.intn(8), Count: 1 + r.intn(2), BaseRev: baseRev}
	default:
		return Op{Type: OpDeleteCols, Sheet: "s1", Index: r.intn(8), Count: 1 + r.intn(2), BaseRev: baseRev}
	}
}

// TestConvergencePropertyManyTrials drives many randomized interleavings of
// ops from several clients (each with a stale-ish base rev) and asserts that
// replaying the server op-log on a fresh workbook reproduces the server state.
// This is the core correctness proof for Transform + Apply + Submit.
func TestConvergencePropertyManyTrials(t *testing.T) {
	for trial := 0; trial < 200; trial++ {
		r := &lcg{state: uint64(trial)*2654435761 + 1}
		w := NewWorkbook()
		w.AddSheet("s1", "Sheet1")
		d := NewDocument(w)

		const clients = 3
		seen := make([]int, clients) // each client's last-seen rev
		for step := 0; step < 30; step++ {
			c := r.intn(clients)
			base := seen[c] + r.intn(d.Head()-seen[c]+1) // somewhere in [seen[c], head]
			if base > d.Head() {
				base = d.Head()
			}
			op := randomOp(r, base)
			if _, err := d.Submit(op); err != nil {
				t.Fatalf("trial %d step %d submit: %v", trial, step, err)
			}
			seen[c] = d.Head()
		}

		replay := NewWorkbook()
		replay.AddSheet("s1", "Sheet1")
		for i, op := range d.Log() {
			if err := replay.Apply(op); err != nil {
				t.Fatalf("trial %d replay op %d: %v", trial, i, err)
			}
		}
		if !workbooksEqual(replay, d.Workbook()) {
			t.Fatalf("trial %d: replay diverged from server state", trial)
		}
	}
}
