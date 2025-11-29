package pad

import (
	"errors"
	"math"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	utils2 "github.com/ether/etherpad-go/admin/src/utils"
	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	"github.com/ether/etherpad-go/lib/models/revision"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/utils"
)

type Pad struct {
	db             db.DataStore
	authorManager  *author.Manager
	Id             string
	ChatHead       int
	Head           int
	PublicStatus   bool
	SavedRevisions []revision.SavedRevision
	Pool           apool.APool
	AText          apool.AText
	hook           *hooks.Hook
}

func NewPad(id string, db db.DataStore, hook *hooks.Hook) Pad {
	p := new(Pad)
	p.Id = id
	p.db = db
	p.Pool = apool.NewAPool()
	p.Head = -1
	p.ChatHead = -1
	p.PublicStatus = false
	p.SavedRevisions = make([]revision.SavedRevision, 0)
	p.hook = hook

	p.AText = changeset.MakeAText("\n", nil)
	return *p
}

func (p *Pad) Check() error {
	if p.Id != strings.TrimSpace(p.Id) {
		return errors.New("pad id contains leading or trailing whitespace")
	}

	head := p.Head
	if head < 0 {
		return errors.New("pad head revision is negative")
	}
	if p.getSavedRevisionNumber() != len(p.SavedRevisions) {
		return errors.New("pad saved revisions count mismatch")
	}

	preSavedRev := -1
	for _, rev := range p.SavedRevisions {
		if rev.RevNum <= preSavedRev {
			return errors.New("pad saved revisions are not in ascending order")
		}
		preSavedRev = rev.RevNum
	}

	padPool := p.Pool
	if err := padPool.Check(); err != nil {
		return errors.New("pad pool check failed: " + err.Error())
	}

	authorIds := map[string]struct{}{}
	padPool.EachAttrib(func(attrib apool.Attribute) {
		if attrib.Key == "author" && attrib.Value != "" {
			authorIds[attrib.Value] = struct{}{}
		}
	})

	revs := make([]revision.Revision, 0)
	for r := 0; r < head+1; r++ {
		isKeyRev := r == p.getKeyRevisionNumber(r)
		revChangeset, err := p.getRevisionChangeset(r)
		if err != nil {
			return errors.New("pad revision " + string(rune(r)) + " retrieval failed: " + err.Error())
		}
		revAuthor, err := p.GetRevisionAuthor(r)
		if err != nil {
			return errors.New("pad revision " + string(rune(r)) + " author retrieval failed: " + err.Error())
		}
		revTimestamp, err := p.GetRevisionDate(r)
		if err != nil {
			return errors.New("pad revision " + string(rune(r)) + " timestamp retrieval failed: " + err.Error())
		}

		atext, err := p.getKeyRevisionAText(r)
		if err != nil {
			return errors.New("pad revision " + string(rune(r)) + " atext retrieval failed: " + err.Error())
		}

		revs = append(revs, utils2.CreateRevision(*revChangeset, *revTimestamp, isKeyRev, revAuthor, *atext, padPool))
	}

	atext := changeset.MakeAText("\n", nil)
	for _, rev := range revs {
		if rev.Meta.Author != nil && *rev.Meta.Author != "" {
			authorIds[*rev.Meta.Author] = struct{}{}
		}
		if rev.Meta.Timestamp < 0 {
			return errors.New("pad revision has negative timestamp")
		}

		if rev.Changeset == "" {
			return errors.New("pad revision has empty changeset")
		}
		if _, err := changeset.CheckRep(rev.Changeset); err != nil {
			return errors.New("pad revision has invalid changeset: " + err.Error())
		}
		unpackedChangeset, err := changeset.Unpack(rev.Changeset)
		if err != nil {
			return errors.New("pad revision has invalid changeset: " + err.Error())
		}

		text := atext.Text

		deserializedOps, err := changeset.DeserializeOps(unpackedChangeset.Ops)
		if err != nil {
			return errors.New("pad revision has invalid changeset ops: " + err.Error())
		}
		for _, op := range *deserializedOps {
			if op.OpCode == "=" || op.OpCode == "-" {
				if text == "" || utf8.RuneCountInString(text) < op.Chars {
					return errors.New("pad revision has changeset op exceeding atext length")
				}
				consumed := text[0:op.Chars]
				nlines := utils.CountLines(consumed, '\n')
				println(nlines, op.Lines)
				if nlines != op.Lines {
					return errors.New("pad revision has changeset op with mismatched lines count")
				}
				if op.Lines > 0 {
					if !strings.HasSuffix(consumed, "\n") {
						return errors.New("pad revision has changeset op with invalid lines content")
					}
				}
				text = text[op.Chars:]
			}
			fromString := changeset.FromString(op.Attribs, &padPool)
			if op.Attribs != fromString.String() {
				return errors.New("pad revision has invalid changeset attribs")
			}
		}
		optAtext, err := changeset.ApplyToAText(rev.Changeset, atext, padPool)
		if err != nil {
			return errors.New("pad revision changeset application failed: " + err.Error())
		}

		atext = *optAtext

		if rev.Meta.IsKeyRev {
			if !apool.ATextsEqual(*rev.Meta.Atext, atext) {
				return errors.New("pad key revision atext does not match computed atext")
			}
		}
	}

	if p.Text() != atext.Text {
		return errors.New("pad atext does not match computed atext")
	}

	if !apool.ATextsEqual(p.AText, atext) {
		return errors.New("pad atext does not match computed atext")
	}

	allAuthors := p.GetAllAuthors()
	for authorId := range authorIds {
		found := false
		for _, aId := range allAuthors {
			if aId == authorId {
				found = true
				break
			}
		}
		if !found {
			return errors.New("pad contains reference to unknown author id: " + authorId)
		}
	}

	if p.ChatHead < -1 {
		return errors.New("pad chat head is negative")
	}

	chatMsgs, err := p.GetChatMessages(0, p.ChatHead)
	if err != nil {
		return errors.New("pad chat messages retrieval failed: " + err.Error())
	}
	for _, msg := range *chatMsgs {
		if *msg.Time < 0 {
			return errors.New("pad chat message has negative timestamp")
		}
		if msg.AuthorId == nil {
			return errors.New("pad chat message has nil author id")
		}
		if msg.PadId != p.Id {
			return errors.New("pad chat message has mismatched pad id")
		}
	}
	return nil
}

func (p *Pad) AppendChatMessage(authorId *string, timestamp int64, text string) int {
	p.ChatHead = p.ChatHead + 1
	err := p.db.SaveChatMessage(p.Id, p.ChatHead, authorId, timestamp, text)
	if err != nil {
		println("Error saving chat message:", err.Error())
	}
	if err := p.db.SaveChatHeadOfPad(p.Id, p.ChatHead); err != nil {
		println("Error saving chat head of pad:", err.Error())
	}

	return p.ChatHead
}

func (p *Pad) RemoveAllChats() error {
	return p.db.RemoveChat(p.Id)
}

func (p *Pad) RemoveAllSavedRevisions() error {
	return p.db.RemoveRevisionsOfPad(p.Id)
}

func (p *Pad) getKeyRevisionNumber(revNum int) int {
	return int(math.Floor(float64(revNum/100)) * 100)
}

func (p *Pad) getKeyRevisionAText(revNum int) (*apool.AText, error) {
	var rev, err = p.db.GetRevision(p.Id, revNum)
	if err != nil {
		return nil, err
	}

	return &rev.AText, err
}

func (p *Pad) Remove() error {
	padId := p.Id
	// Kick session is done in ws package to avoid circular import
	if utils.RuneIndex(padId, "$") >= 0 {
		indexOfDollar := utils.RuneIndex(padId, "$")
		groupId := padId[0:indexOfDollar]
		groupVal, err := p.db.GetGroup(groupId)
		if err != nil {
			return err
		}
		// TODO remove pad from group pads list
		println("Removing group:", groupVal)
	}
	// Actual code was moved to padManager to avoid circular import
	return nil
}

func (p *Pad) GetRevisionAuthor(revNum int) (*string, error) {
	rev, err := p.db.GetRevision(p.Id, revNum)
	if err != nil {
		return nil, err
	}
	if rev.AuthorId == nil {
		return nil, errors.New("invalid rev id")
	}
	return rev.AuthorId, nil
}

func (p *Pad) getRevisionChangeset(revNum int) (*string, error) {
	var rev, err = p.db.GetRevision(p.Id, revNum)
	if err != nil {
		return nil, err
	}

	return &rev.Changeset, err
}

func (p *Pad) GetInternalRevisionAText(targetRev int) *apool.AText {
	var keyRev = p.getKeyRevisionNumber(targetRev)
	var headRev = p.getHeadRevisionNumber()
	if targetRev > headRev {
		targetRev = headRev
	}
	var keyAText, err = p.getKeyRevisionAText(keyRev)
	if err != nil {
		return nil
	}
	var atext = *keyAText
	for i := keyRev + 1; i <= targetRev; i++ {
		var changesetPad, err = p.getRevisionChangeset(i)
		if err != nil {
			return nil
		}
		optAtext, err := changeset.ApplyToAText(*changesetPad, atext, *p.apool())
		if err != nil {
			return nil
		}
		atext = *optAtext
	}
	return &atext
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

func (p *Pad) Init(text *string, author *string, authorManager *author.Manager) error {
	p.authorManager = authorManager
	if author == nil {
		author = new(string)
		*author = ""
	}

	var pad, err = p.db.GetPad(p.Id)

	if err == nil {
		var _, err = p.db.GetRevision(p.Id, pad.RevNum)
		if err != nil {
			return errors.New("pad data is corrupted: missing revision")
		}

		mapDBPadToModel(pad, p)
	} else {
		if text == nil {
			var padDefaultText = "text"
			text = &settings.Displayed.DefaultPadText
			var context = DefaultContent{
				AuthorId: author,
				Type:     &padDefaultText,
				Content:  text,
				Pad:      p,
			}
			p.hook.ExecuteHooks(hooks.PadDefaultContentString, &context)
			text = context.Content

			if *context.Type != "text" {
				return errors.New("unsupported content type" + *context.Type)
			}
		}

		var firstChangeset, _ = changeset.MakeSplice("\n", 0, 0, *text, nil, nil)
		p.AppendRevision(firstChangeset, author)
	}

	p.hook.ExecuteHooks(hooks.PadLoadString, Load{
		Pad: p,
	})
	return nil
}

func (p *Pad) SetText(newText string, authorId *string) error {
	var authorIdToSend string
	if authorId == nil {
		authorIdToSend = ""
	} else {
		authorIdToSend = *authorId
	}

	return p.SpliceText(0, utf8.RuneCountInString(p.Text()), newText, &authorIdToSend)
}

func (p *Pad) SpliceText(start int, ndel int, ins string, authorId *string) error {
	if start < 0 {
		return errors.New("start index must not be negative")
	}
	if ndel < 0 {
		return errors.New("characters to delete must be non-negative")
	}
	var orig = p.Text()
	if !strings.HasSuffix(orig, "\n") {
		return errors.New("text must end with a newline")
	}
	if start+ndel > utf8.RuneCountInString(orig) {
		return errors.New("splice out of bounds")
	}

	ins = *CleanText(ins)
	var willEndWithNewLine = start+ndel < utf8.RuneCountInString(orig) || strings.HasSuffix(ins, "\n") || (ins == "" && start > 0 && orig[start-1] == '\n')
	if !willEndWithNewLine {
		ins += "\n"
	}
	if ndel == 0 && utf8.RuneCountInString(ins) == 0 {
		return nil
	}
	var changesetFromSplice, err = changeset.MakeSplice(orig, start, ndel, ins, nil, nil)
	if err != nil {
		return err
	}
	p.AppendRevision(changesetFromSplice, authorId)
	return nil
}

func (p *Pad) GetRevision(revNumber int) (*db2.PadSingleRevision, error) {
	return p.db.GetRevision(p.Id, revNumber)
}

func (p *Pad) GetRevisions(start int, end int) (*[]db2.PadSingleRevision, error) {
	return p.db.GetRevisions(p.Id, start, end)
}

func (p *Pad) Save() error {
	return p.db.CreatePad(p.Id, db2.PadDB{
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
	return len(p.SavedRevisions)
}

func (p *Pad) getSavedRevisionsList() []int {
	var savedRevisions = make([]int, len(p.SavedRevisions))

	for i, rev := range p.SavedRevisions {
		savedRevisions[i] = rev.RevNum
	}

	slices.Sort(savedRevisions)
	return savedRevisions
}

func (p *Pad) GetRevisionDate(rev int) (*int64, error) {
	revision, err := p.db.GetRevision(p.Id, rev)

	if err != nil {
		return nil, err
	}

	return &revision.Timestamp, nil
}

func (p *Pad) getPublicStatus() bool {
	return p.PublicStatus
}

func (p *Pad) AppendRevision(cs string, authorId *string) int {
	if authorId == nil {
		authorId = new(string)
		*authorId = ""
	}
	var newAText, err = changeset.ApplyToAText(cs, p.AText, p.Pool)

	if err != nil {
		println("Error applying changeset to atext:", err.Error())
		return p.Head
	}

	if newAText.Text == p.AText.Text && newAText.Attribs == p.AText.Attribs && p.Head != -1 {
		return p.Head
	}

	apool.CopyAText(*newAText, &p.AText)

	p.Head++

	var newRev = p.Head

	if authorId != nil {
		p.Pool.PutAttrib(apool.Attribute{
			Key:   "author",
			Value: *authorId,
		}, nil)
	}

	// Save pad
	p.Save()

	var poolToUse apool.APool
	var atextToUse apool.AText

	poolToUse = p.Pool
	atextToUse = p.AText

	err = p.db.SaveRevision(p.Id, newRev, cs, atextToUse, poolToUse, authorId, time.Now().UnixNano()/int64(time.Millisecond))

	if err != nil {
		println("Error saving revision", err.Error())
	}

	if authorId != nil {
		var clonedAuthorId = *authorId
		if clonedAuthorId != "" {
			p.authorManager.AddPad(*authorId, p.Id)
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

func (p *Pad) GetChatMessages(start int, end int) (*[]db2.ChatMessageDBWithDisplayName, error) {
	return p.db.GetChatsOfPad(p.Id, start, end)
}
