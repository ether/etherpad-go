package sheetdoc

import (
	"sync"

	"github.com/ether/etherpad-go/lib/sheet"
)

// Earlier versions are rebuilt by replaying the stored ops. Workbooks at
// every checkpointEvery-th revision are kept in memory, so showing another
// version replays at most that many ops (per sheet at most maxCheckpoints,
// for at most maxHistorySheets sheets).
const (
	checkpointEvery  = 200
	maxCheckpoints   = 32
	maxHistorySheets = 16
	replayBatch      = 1000
)

type sheetCheckpoints struct {
	epoch int
	// baseSet: the base was looked up; points[0] is nil when it is not
	// known.
	baseSet bool
	points  map[int]*sheet.Workbook
}

type checkpoints struct {
	mu     sync.Mutex
	sheets map[string]*sheetCheckpoints
}

// of returns padId's checkpoints of epoch. The caller holds c.mu.
func (c *checkpoints) of(padId string, epoch int) *sheetCheckpoints {
	if c.sheets == nil {
		c.sheets = map[string]*sheetCheckpoints{}
	}
	s := c.sheets[padId]
	if s == nil || s.epoch != epoch {
		if s == nil && len(c.sheets) >= maxHistorySheets {
			for k := range c.sheets {
				delete(c.sheets, k)
				break
			}
		}
		s = &sheetCheckpoints{epoch: epoch, points: map[int]*sheet.Workbook{}}
		c.sheets[padId] = s
	}
	return s
}

func (c *checkpoints) reset(padId string, epoch int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.of(padId, epoch)
	s.baseSet = false
	clear(s.points)
}

// base returns the base when it was looked up (ok); known is false when it
// is not known.
func (c *checkpoints) base(padId string, epoch int) (wb *sheet.Workbook, known, ok bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.of(padId, epoch)
	if !s.baseSet {
		return nil, false, false
	}
	return s.points[0], s.points[0] != nil, true
}

func (c *checkpoints) setBase(padId string, epoch int, wb *sheet.Workbook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.of(padId, epoch)
	s.baseSet = true
	if wb == nil {
		clear(s.points)
		return
	}
	s.points[0] = wb.Clone()
}

// keep stores a copy of wb as the workbook at rev.
func (c *checkpoints) keep(padId string, epoch, rev int, wb *sheet.Workbook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.of(padId, epoch)
	if _, ok := s.points[rev]; ok || len(s.points) >= maxCheckpoints {
		return
	}
	s.points[rev] = wb.Clone()
}

// nearest returns a copy of the latest kept workbook at or before rev and
// its revision; nil if the base is not known.
func (c *checkpoints) nearest(padId string, epoch, rev int) (int, *sheet.Workbook) {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.of(padId, epoch)
	best := -1
	for r := range s.points {
		if r <= rev && r > best {
			best = r
		}
	}
	if best < 0 || s.points[best] == nil {
		return 0, nil
	}
	return best, s.points[best].Clone()
}
