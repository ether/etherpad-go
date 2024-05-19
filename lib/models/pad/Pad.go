package pad

import (
	"github.com/ether/etherpad-go/lib/db"
	"regexp"
	"slices"
	"strings"
)


var regex1 *regexp.Regexp
var regex2 *regexp.Regexp
var regex3 *regexp.Regexp
var regex4 *regexp.Regexp

func init() {
	regex1, _ = regexp.Compile("\r\n")
	regex2,_ = regexp.Compile("\r")
	regex3,_ = regexp.Compile("\t")
	regex4,_ = regexp.Compile("\xa0")

}


type Pad struct {
	db             db.DataStore
	Id             string
	ChatHead       int
	Head           int
	PublicStatus   bool
	savedRevisions []Revision
	Pool           APool
}

func NewPad(id string) Pad {
	p := new(Pad)
	p.Id = id
	p.Pool = *NewAPool()
	p.Head = -1
	p.ChatHead = -1
	p.PublicStatus = false
	p.savedRevisions = make([]Revision, 0)

	return *p
}

func (p *Pad) apool() *APool {
	return &p.Pool
}


func cleanText(context string) *string {
	var newStr = regex1.ReplaceAllString(context, "\n")
	newStr = regex2.ReplaceAllString(newStr, "\n")
	newStr = regex3.ReplaceAllString(newStr, "        ")
	newStr = regex4.ReplaceAllString(newStr, " ")
	return &newStr
}

func (p *Pad) Init(text *string, author string) {
	var pad, err = p.db.GetPad(p.Id)

	if err == nil {
		if pad.Pool != nil {
			p.Pool = *pad.Pool
		}
	} else {
		if text == nil {
			var context = "Pad.Init"
			text = cleanText(context)
		}
		var firstChangeset = 

	}

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
