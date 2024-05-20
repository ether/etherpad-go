package pad

import (
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/utils"
	"regexp"
	"slices"
)

var regex1 *regexp.Regexp
var regex2 *regexp.Regexp
var regex3 *regexp.Regexp
var regex4 *regexp.Regexp

var authorManager author.Manager

func init() {
	regex1, _ = regexp.Compile("\r\n")
	regex2, _ = regexp.Compile("\r")
	regex3, _ = regexp.Compile("\t")
	regex4, _ = regexp.Compile("\xa0")
	authorManager = author.Manager{
		Db: utils.DataStore,
	}
}

type Pad struct {
	db             db.DataStore
	Id             string
	ChatHead       int
	Head           int
	PublicStatus   bool
	savedRevisions []Revision
	Pool           apool.APool
	AText          apool.AText
}

func NewPad(id string) Pad {
	p := new(Pad)
	p.Id = id
	p.Pool = *apool.NewAPool()
	p.Head = -1
	p.ChatHead = -1
	p.PublicStatus = false
	p.savedRevisions = make([]Revision, 0)
	p.AText = changeset.MakeAText("\n", nil)
	return *p
}

func (p *Pad) apool() *apool.APool {
	return &p.Pool
}

func cleanText(context string) *string {
	var newStr = regex1.ReplaceAllString(context, "\n")
	newStr = regex2.ReplaceAllString(newStr, "\n")
	newStr = regex3.ReplaceAllString(newStr, "        ")
	newStr = regex4.ReplaceAllString(newStr, " ")
	return &newStr
}

func (p *Pad) Init(text *string, author *string) {
	if author == nil {
		author = new(string)
		*author = ""
	}

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
		var firstChangeset, _ = changeset.MakeSplice("\n", 0, 0, *text, nil, nil)
		p.appendRevision(firstChangeset, author)
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

func (p *Pad) appendRevision(cs string, authorId *string) int {
	if authorId == nil {
		authorId = new(string)
		*authorId = ""
	}
	var newAText = changeset.ApplyToAText(cs, p.AText, p.Pool)

	if newAText.Text == p.AText.Text && newAText.Attribs == p.AText.Attribs && p.Head != -1 {
		return p.Head
	}

	apool.CopyAText(newAText, p.AText)

	p.Head++

	if authorId != nil {
		p.Pool.PutAttrib(apool.Attribute{
			Key:   "author",
			Value: *authorId,
		}, nil)
	}

	p.db.SaveRevision(p.Id, p.Head, cs, *p.apool())

	if authorId != nil {
		var clonedAuthorId = *authorId
		if clonedAuthorId != "" {
			authorManager.AddPad(*authorId, p.Id)
		}
	}

	return p.Head
}
