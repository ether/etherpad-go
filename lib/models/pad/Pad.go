package pad

import "slices"

type Pad struct {
	Id             string
	ChatHead       int
	Head           int
	PublicStatus   bool
	savedRevisions []Revision
	Pool           APool
}

func NewPad(id string) {
	p := new(Pad)
	p.Id = id
	p.Pool = *NewAPool()
	p.Head = -1
	p.ChatHead = -1
	p.PublicStatus = false
	p.savedRevisions = make([]Revision, 0)
}

func (p *Pad) apool() *APool {
	return &p.Pool
}

func (p *Pad) getHeadRevisionNumber() int {
	return p.Head
}

func (p *Pad) getSavedRevisionNumber() int {
	return len(p.savedRevisions)
}

func (p *Pad) getSavedRevisionsList() []int {
	var savedRevisions = make([]int, len(p.savedRevisions))

	for i, rev := range p.savedRevisions {
		savedRevisions[i] = rev.revNum
	}

	slices.Sort(savedRevisions)
	return savedRevisions
}

func (p *Pad) getPublicStatus() bool {
	return p.PublicStatus
}

func (p *Pad) appendRevision(rev Revision) {
	p.savedRevisions = append(p.savedRevisions, rev)
}
