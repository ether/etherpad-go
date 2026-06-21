package sheet

import "fmt"

// Document is the authoritative server-side state for one sheet document:
// the current Workbook, a monotonically growing op-log, and the head revision.
// It is NOT goroutine-safe; the per-document serialization goroutine (plan 2c)
// provides the total order, exactly as the text pad channel does.
type Document struct {
	wb   *Workbook
	log  []Op // log[i] is the op that advanced head from i to i+1
	head int
}

func NewDocument(wb *Workbook) *Document {
	return &Document{wb: wb, log: []Op{}, head: 0}
}

func (d *Document) Head() int           { return d.head }
func (d *Document) Workbook() *Workbook { return d.wb }
func (d *Document) Log() []Op           { return d.log }

// Submit rebases an op composed against op.BaseRev past every op applied since
// then, applies it, appends the rebased op to the log, and returns the new
// head revision. The rebased op (not the original) is logged so replay is exact.
func (d *Document) Submit(op Op) (int, error) {
	if err := op.Validate(); err != nil {
		return 0, err
	}
	if op.BaseRev < 0 || op.BaseRev > d.head {
		return 0, fmt.Errorf("submit: baseRev %d out of range (head %d)", op.BaseRev, d.head)
	}
	rebased := op
	for i := op.BaseRev; i < d.head; i++ {
		rebased = Transform(rebased, d.log[i])
	}
	rebased.BaseRev = d.head
	if err := d.wb.Apply(rebased); err != nil {
		return 0, err
	}
	d.log = append(d.log, rebased)
	d.head++
	return d.head, nil
}
