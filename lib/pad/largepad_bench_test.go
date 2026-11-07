package pad

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/db"
)

func benchmarkAppendToLargePad(b *testing.B, store db.DataStore) {
	m := newInstanceB(store)
	padID := "large"
	unlock := m.LockPad(padID)
	p, err := m.GetPad(padID, nil, nil)
	if err != nil {
		b.Fatal(err)
	}
	if err := p.SpliceText(0, 0, strings.Repeat("lorem ipsum dolor sit amet lorem ipsum dolor sit amet lorem\n", 1_000_000/62), nil); err != nil {
		b.Fatal(err)
	}
	unlock()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		unlock := m.LockPad(padID)
		p, _ := m.GetPad(padID, nil, nil)
		if err := p.SpliceText(utf8.RuneCountInString(p.Text())-1, 0, "x\n", nil); err != nil {
			b.Fatal(err)
		}
		unlock()
	}
}

func BenchmarkAppendToLargePadMemory(b *testing.B) {
	benchmarkAppendToLargePad(b, db.NewMemoryDataStore())
}

func BenchmarkAppendToLargePadSQLite(b *testing.B) {
	s, err := db.NewSQLiteDB(b.TempDir() + "/bench.db")
	if err != nil {
		b.Fatal(err)
	}
	benchmarkAppendToLargePad(b, s)
}
