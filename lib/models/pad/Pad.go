package pad

import (
	"errors"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
	"slices"
	"strings"
	"time"
)

var authorManager author.Manager

func init() {
	authorManager = author.NewManager()
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
	p.db = utils.GetDB()
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

func (p *Pad) Text() string {
	return p.AText.Text
}

func CleanText(context string) *string {
	context = strings.ReplaceAll(context, "\r\n", "\n")
	context = strings.ReplaceAll(context, "\r", "\n")
	context = strings.ReplaceAll(context, "\t", "        ")
	context = strings.ReplaceAll(context, "\xa0", " ")
	return &context
}

func (p *Pad) Init(text *string, author *string) error {
	if author == nil {
		author = new(string)
		*author = ""
	}

	var pad, err = p.db.GetPad(p.Id)

	if err == nil {
		var padMetaData = pad.SavedRevisions[pad.RevNum].PadDBMeta
		p.Pool = *padMetaData.Pool
	} else {
		if text == nil {
			var padDefaultText = "text"
			text = &settings.SettingsDisplayed.DefaultPadText
			var context = DefaultContent{
				AuthorId: author,
				Type:     &padDefaultText,
				Content:  text,
				Pad:      p,
			}
			hooks.HookInstance.ExecuteHooks(hooks.PadDefaultContentString, context)

			if *context.Type != "text" {
				return errors.New("unsupported content type" + *context.Type)
			}

		}

		var firstChangeset, _ = changeset.MakeSplice("\n", 0, 0, *text, nil, nil)
		p.AppendRevision(firstChangeset, author)
		p.save()
	}

	hooks.HookInstance.ExecuteHooks(hooks.PadLoadString, Load{
		Pad: p,
	})
	return nil
}

func (p *Pad) GetRevision(revNumber int) (*db2.PadSingleRevision, error) {
	return p.db.GetRevision(p.Id, revNumber)
}

func (p *Pad) save() {
	p.db.CreatePad(p.Id, db2.PadDB{
		SavedRevisions: make(map[int]db2.PadRevision),
		RevNum:         p.Head,
		Pool:           p.Pool.ToJsonable(),
		AText:          p.AText,
	})
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

func (p *Pad) GetRevisionDate(rev int) int {
	revision, _ := p.db.GetRevision(p.Id, rev)

	return revision.Timestamp
}

func (p *Pad) getPublicStatus() bool {
	return p.PublicStatus
}

func (p *Pad) AppendRevision(cs string, authorId *string) int {
	if authorId == nil {
		authorId = new(string)
		*authorId = ""
	}
	var newAText = changeset.ApplyToAText(cs, p.AText, p.Pool)

	if newAText.Text == p.AText.Text && newAText.Attribs == p.AText.Attribs && p.Head != -1 {
		return p.Head
	}

	apool.CopyAText(newAText, &p.AText)

	p.Head++

	if authorId != nil {
		p.Pool.PutAttrib(apool.Attribute{
			Key:   "author",
			Value: *authorId,
		}, nil)
	}

	// Save pad
	p.save()
	p.db.SaveRevision(p.Id, p.Head, cs, p.AText, *p.apool(), authorId, int(time.Now().UnixNano()/int64(time.Millisecond)))

	if authorId != nil {
		var clonedAuthorId = *authorId
		if clonedAuthorId != "" {
			authorManager.AddPad(*authorId, p.Id)
		}
	}

	return p.Head
}

func (p *Pad) GetAllAuthors() []string {
	var authorIds = make([]string, 0)

	for k, v := range p.Pool.NumToAttrib {
		if p.Pool.NumToAttrib[k].Key == "author" && p.Pool.NumToAttrib[k].Value != "" {
			authorIds = append(authorIds, v.Value)
		}
	}
	return authorIds
}

func (p *Pad) GetPadMetaData(revNum int) *db2.PadMetaData {
	meta, err := p.db.GetPadMetaData(p.Id, revNum)

	if err != nil {
		return nil
	}

	return meta
}
