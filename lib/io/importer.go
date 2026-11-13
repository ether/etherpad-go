package io

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ether/etherpad-go/lib/apool"
	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/changeset"
	"github.com/ether/etherpad-go/lib/db"
	"github.com/ether/etherpad-go/lib/hooks"
	"github.com/ether/etherpad-go/lib/hooks/events"
	db2 "github.com/ether/etherpad-go/lib/models/db"
	padModel "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/pad"
	"github.com/ether/etherpad-go/lib/utils"
	"github.com/ledongthuc/pdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"go.uber.org/zap"
	"golang.org/x/net/html"
)

type Importer struct {
	padManager    *pad.Manager
	authorManager *author.Manager
	db            db.DataStore
	logger        *zap.SugaredLogger
	hooks         *hooks.Hook
}

func NewImporter(padManager *pad.Manager, authorManager *author.Manager, db db.DataStore, logger *zap.SugaredLogger, hooks *hooks.Hook) *Importer {
	return &Importer{
		padManager:    padManager,
		authorManager: authorManager,
		db:            db,
		logger:        logger,
		hooks:         hooks,
	}
}

var validHexColorRegex = regexp.MustCompile(`^#(?:[0-9a-fA-F]{3}|[0-9a-fA-F]{6})$`)

func sanitizeImportedColorId(rawColorId any) string {
	switch v := rawColorId.(type) {
	case string:
		if validHexColorRegex.MatchString(v) {
			return v
		}
		if idx, err := strconv.Atoi(v); err == nil && idx >= 0 && idx < len(utils.ColorPalette) {
			return utils.ColorPalette[idx]
		}
	case float64:
		idx := int(v)
		if float64(idx) == v && idx >= 0 && idx < len(utils.ColorPalette) {
			return utils.ColorPalette[idx]
		}
	case int:
		if v >= 0 && v < len(utils.ColorPalette) {
			return utils.ColorPalette[v]
		}
	}
	return utils.ColorPalette[rand.Intn(len(utils.ColorPalette))]
}

// ErrImportChangesCreator is returned by SetPadRaw when someone other than
// the pad's creator imports a .etherpad file whose revision 0 names a
// different author. The rev-0 author is the creator, who may delete the pad,
// so handing that role over is reserved to the creator (etherpad-lite #8420).
var ErrImportChangesCreator = errors.New("only the pad creator can import a file with a different creator")

type EtherpadImport struct {
	rawData map[string]json.RawMessage
}

func (i *Importer) SetPadRaw(padId string, content []byte, authorId string) error {
	var rawData map[string]json.RawMessage
	if err := json.Unmarshal(content, &rawData); err != nil {
		return errors.New("invalid etherpad JSON: " + err.Error())
	}

	// Every record must belong to one source pad: a file that mixes pads
	// ("pad:a" with "pad:b:revs:0", or an author of another pad) is
	// rejected as a whole, as in etherpad-lite.
	if _, err := sourcePadID(rawData); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEtherpadFile, err)
	}

	var padData PadData
	var foundPadData bool

	// etherpad-lite stores the pad record under "pad:<id>"; older etherpad-go
	// exports used "pad:<id>:". Accept both.
	padKeyRegex := regexp.MustCompile(`^pad:[^:]+:?$`)
	for key, value := range rawData {
		if padKeyRegex.MatchString(key) {
			// Try to parse as PadData
			if err := json.Unmarshal(value, &padData); err == nil {
				foundPadData = true
				if i.logger != nil {
					i.logger.Infof("Found pad data at key: %s", key)
				}
				break
			}
		}
	}

	if !foundPadData {
		return errors.New("no pad data found in etherpad file")
	}

	// Fire importEtherpad hook — observational; plugins may inspect or augment Data.
	if i.hooks != nil {
		var rawMap map[string]any
		if err := json.Unmarshal(content, &rawMap); err != nil && i.logger != nil {
			i.logger.Warnf("importEtherpad hook: could not unmarshal rawMap for hook: %v", err)
		}

		srcPadId := ""
		for key := range rawData {
			if padKeyRegex.MatchString(key) {
				// Extract the pad name: "pad:<name>:"
				trimmed := strings.TrimPrefix(key, "pad:")
				trimmed = strings.TrimSuffix(trimmed, ":")
				srcPadId = trimmed
				break
			}
		}

		i.hooks.ExecuteImportEtherpadHooks(&events.ImportEtherpadContext{
			PadId:    padId,
			SrcPadId: srcPadId,
			Data:     rawMap,
		})
	}

	// Security metadata comes from the target pad, never from the file: an
	// importer only needs write access, so the file must not be able to make
	// a private group pad public, reset its read-only link or make someone
	// else its creator (etherpad-lite #8420).
	targetPad, targetErr := i.db.GetPad(padId)
	targetExists := targetErr == nil && targetPad != nil
	creatorId := authorId
	if rev0, err := i.db.GetRevision(padId, 0); err == nil && rev0 != nil {
		creatorId = ""
		if rev0.AuthorId != nil {
			creatorId = *rev0.AuthorId
		}
	}
	importerIsCreator := !targetExists || (authorId != "" && creatorId == authorId)

	revisions := make(map[int]Revision)
	revisionRegex := regexp.MustCompile(`^pad:[^:]+:revs:(\d+)$`)
	for key, value := range rawData {
		matches := revisionRegex.FindStringSubmatch(key)
		if matches != nil {
			revNum, _ := strconv.Atoi(matches[1])
			var rev Revision
			if err := json.Unmarshal(value, &rev); err == nil {
				revisions[revNum] = rev
			}
		}
	}
	// The file's pad-wide settings only count when the creator imports it;
	// otherwise the target pad keeps its own.
	var settingsJSON *string
	if importerIsCreator {
		if len(padData.PadSettings) > 0 && string(padData.PadSettings) != "null" {
			s := string(padData.PadSettings)
			settingsJSON = &s
		}
	} else if targetExists && targetPad.Settings != nil {
		s := *targetPad.Settings
		settingsJSON = &s
	}

	if rev0, ok := revisions[0]; ok && !importerIsCreator {
		fileCreator := ""
		if rev0.Meta.Author != nil {
			fileCreator = *rev0.Meta.Author
		}
		if fileCreator != creatorId {
			return ErrImportChangesCreator
		}
	}

	publicStatus := false
	var readOnlyId *string
	if targetExists {
		publicStatus = targetPad.PublicStatus
		readOnlyId = targetPad.ReadOnlyId
	}

	pool := apool.NewAPool()
	for numStr, attrib := range padData.Pool.NumToAttrib {
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		if len(attrib) >= 2 {
			a := apool.Attribute{Key: attrib[0], Value: attrib[1]}
			pool.NumToAttrib[num] = a
			pool.AttribToNum[a] = num
		}
	}
	pool.NextNum = padData.Pool.NextNum

	imp := &parsedEtherpad{
		pad: db2.PadDB{
			SavedRevisions: make([]db2.SavedRevision, 0),
			Head:           padData.Head,
			Pool:           pool.ToRevDB(),
			ATextText:      padData.AText.Text,
			ATextAttribs:   padData.AText.Attribs,
			CreatedAt:      time.Now(),
			ReadOnlyId:     readOnlyId,
			ID:             padId,
			PublicStatus:   publicStatus,
			Settings:       settingsJSON,
		},
		revisions: revisions,
		authors:   make(map[string]GlobalAuthor),
	}
	pool.EachAttrib(func(a apool.Attribute) {
		if a.Key == "author" && a.Value != "" {
			imp.referencedAuthors = append(imp.referencedAuthors, a.Value)
		}
	})
	for _, rev := range revisions {
		if rev.Meta.Author != nil && *rev.Meta.Author != "" {
			imp.referencedAuthors = append(imp.referencedAuthors, *rev.Meta.Author)
		}
	}

	authorRegex := regexp.MustCompile(`^globalAuthor:(.+)$`)
	chatRegex := regexp.MustCompile(`^pad:[^:]+:chat:(\d+)$`)
	type indexedChat struct {
		idx int
		msg ChatMessage
	}
	var chats []indexedChat
	for key, value := range rawData {
		if m := authorRegex.FindStringSubmatch(key); m != nil {
			var authorData GlobalAuthor
			if err := json.Unmarshal(value, &authorData); err == nil {
				imp.authors[m[1]] = authorData
			}
		} else if m := chatRegex.FindStringSubmatch(key); m != nil {
			idx, _ := strconv.Atoi(m[1])
			var chat ChatMessage
			if err := json.Unmarshal(value, &chat); err == nil {
				chats = append(chats, indexedChat{idx: idx, msg: chat})
				if chat.UserId != nil && *chat.UserId != "" {
					imp.referencedAuthors = append(imp.referencedAuthors, *chat.UserId)
				}
			}
		}
	}
	// Keep the file's chat order and store it contiguously from 0; map
	// iteration order is random.
	sort.Slice(chats, func(a, b int) bool { return chats[a].idx < chats[b].idx })
	for _, c := range chats {
		imp.chats = append(imp.chats, c.msg)
	}
	imp.pad.ChatHead = len(imp.chats) - 1

	// Files from before every insert had to carry an author: attribute
	// those inserts to the system author, as etherpad-lite does, so the
	// pad keeps the invariant that its edits and restores rely on.
	if changed, err := attributeLegacyInserts(imp, &pool); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEtherpadFile, err)
	} else if changed && i.logger != nil {
		i.logger.Infof("Import into %s: attributed unattributed inserts to %s", padId, padModel.SystemAuthorId)
	}

	// Like etherpad-lite, validate the whole import in a scratch store before
	// anything is written: a broken file must be rejected while the target
	// pad is still intact, not leave a half-imported pad behind.
	if err := i.validateImport(padId, imp); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidEtherpadFile, err)
	}

	// The target is replaced. Keep a copy, so a write that fails halfway
	// (database error) puts the old pad back instead of losing it.
	backup, err := snapshotPad(i.db, padId)
	if err != nil {
		return fmt.Errorf("import into %s: saving the current pad first: %w", padId, err)
	}
	if err := i.db.RemovePad(padId); err != nil {
		return fmt.Errorf("import into %s: %w", padId, err)
	}
	if err := i.writeImport(i.db, imp, true); err != nil {
		if backup != nil {
			_ = i.db.RemovePad(padId)
			if rerr := backup.restore(i.db); rerr != nil {
				if i.logger != nil {
					i.logger.Errorf("import into %s failed (%v) and restoring the previous pad failed too: %v", padId, err, rerr)
				}
				return fmt.Errorf("import into %s failed: %w; restoring the previous pad failed: %v", padId, err, rerr)
			}
		}
		return fmt.Errorf("import into %s failed, the previous pad is unchanged: %w", padId, err)
	}
	return nil
}

// attributeLegacyInserts gives every insert op without an author the
// system author and, when it changed anything, recomputes the pad's text and
// the revisions' stored texts from the rewritten changesets. A file that
// needs nothing is left exactly as it is.
func attributeLegacyInserts(imp *parsedEtherpad, pool *apool.APool) (bool, error) {
	revNums := make([]int, 0, len(imp.revisions))
	for n := range imp.revisions {
		revNums = append(revNums, n)
	}
	sort.Ints(revNums)

	changed := false
	rewritten := make(map[int]string, len(revNums))
	for _, n := range revNums {
		cs, err := changeset.Unpack(imp.revisions[n].Changeset)
		if err != nil {
			return false, fmt.Errorf("revision %d: %w", n, err)
		}
		ops, err := changeset.DeserializeOps(cs.Ops)
		if err != nil {
			return false, fmt.Errorf("revision %d: %w", n, err)
		}
		assem := changeset.NewOpAssembler()
		revChanged := false
		for _, op := range *ops {
			if op.OpCode == "+" {
				attribs := changeset.FromString(op.Attribs, pool)
				if v := attribs.Get("author"); v == nil || *v == "" {
					attribs.Set("author", padModel.SystemAuthorId)
					op.Attribs = attribs.String()
					revChanged = true
				}
			}
			assem.Append(op)
		}
		if revChanged {
			changed = true
			rewritten[n] = changeset.Pack(cs.OldLen, cs.NewLen, assem.String(), cs.CharBank)
		}
	}
	if !changed {
		return false, nil
	}

	atext := changeset.MakeAText("\n", nil)
	for _, n := range revNums {
		rev := imp.revisions[n]
		if cs, ok := rewritten[n]; ok {
			rev.Changeset = cs
		}
		next, err := changeset.ApplyToAText(rev.Changeset, atext, *pool)
		if err != nil {
			return false, fmt.Errorf("revision %d: %w", n, err)
		}
		atext = *next
		if rev.Meta.AText != nil {
			rev.Meta.AText = &AText{Text: atext.Text, Attribs: atext.Attribs}
			rev.Meta.Pool = poolForRevision(pool)
		}
		imp.revisions[n] = rev
	}
	imp.pad.ATextText = atext.Text
	imp.pad.ATextAttribs = atext.Attribs
	imp.pad.Pool = pool.ToRevDB()
	imp.referencedAuthors = append(imp.referencedAuthors, padModel.SystemAuthorId)
	return true, nil
}

func poolForRevision(pool *apool.APool) *PoolWithAttribToNum {
	p := pool.ToRevDB()
	return &PoolWithAttribToNum{NumToAttrib: p.NumToAttrib, AttribToNum: p.AttribToNum, NextNum: p.NextNum}
}

// sourcePadID returns the id of the pad the records were exported from and
// checks that every pad record and every author belongs to it.
func sourcePadID(rawData map[string]json.RawMessage) (string, error) {
	src := ""
	check := func(id string) error {
		if src == "" {
			src = id
		}
		if id != src {
			return fmt.Errorf("unexpected pad ID in record: %q, expected %q", id, src)
		}
		return nil
	}
	// Pad records first, so the error names the pad's id, not an author's.
	keys := make([]string, 0, len(rawData))
	for key := range rawData {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		parts := strings.Split(key, ":")
		if parts[0] == "pad" && len(parts) >= 2 {
			if err := check(parts[1]); err != nil {
				return "", err
			}
		}
	}
	for _, key := range keys {
		if !strings.HasPrefix(key, "globalAuthor:") {
			continue
		}
		var a struct {
			PadIDs json.RawMessage `json:"padIDs"`
		}
		if json.Unmarshal(rawData[key], &a) != nil || len(a.PadIDs) == 0 || string(a.PadIDs) == "null" {
			continue
		}
		var padIDs string
		if json.Unmarshal(a.PadIDs, &padIDs) != nil {
			return "", fmt.Errorf("%s: padIDs is not a string", key)
		}
		if padIDs != "" {
			if err := check(padIDs); err != nil {
				return "", err
			}
		}
	}
	return src, nil
}

// padSnapshot is a pad's stored rows, for putting it back.
type padSnapshot struct {
	pad       db2.PadDB
	revisions []db2.PadSingleRevision
	chats     []db2.ChatMessageDBWithDisplayName
	chatHead  int
}

// snapshotPad reads the pad's rows; nil when there is no such pad.
func snapshotPad(store db.DataStore, padId string) (*padSnapshot, error) {
	row, err := store.GetPad(padId)
	if err != nil || row == nil {
		return nil, nil
	}
	snap := &padSnapshot{pad: *row, chatHead: row.ChatHead}
	if row.Head >= 0 {
		revs, err := store.GetRevisions(padId, 0, row.Head)
		if err != nil {
			return nil, err
		}
		if revs != nil {
			snap.revisions = *revs
		}
	}
	if row.ChatHead >= 0 {
		chats, err := store.GetChatsOfPad(padId, 0, row.ChatHead)
		if err != nil {
			return nil, err
		}
		if chats != nil {
			snap.chats = *chats
		}
	}
	return snap, nil
}

func (s *padSnapshot) restore(store db.DataStore) error {
	if err := store.CreatePad(s.pad.ID, s.pad); err != nil {
		return err
	}
	for _, rev := range s.revisions {
		pool := db2.RevPool{}
		if rev.Pool != nil {
			pool = *rev.Pool
		}
		if err := store.SaveRevision(s.pad.ID, rev.RevNum, rev.Changeset, rev.AText, pool, rev.AuthorId, rev.Timestamp); err != nil {
			return err
		}
	}
	for _, c := range s.chats {
		var ts int64
		if c.Time != nil {
			ts = *c.Time
		}
		if err := store.SaveChatMessage(s.pad.ID, c.Head, c.AuthorId, ts, c.Message); err != nil {
			return err
		}
	}
	if s.chatHead >= 0 {
		return store.SaveChatHeadOfPad(s.pad.ID, s.chatHead)
	}
	return nil
}

// ErrInvalidEtherpadFile wraps every reason a .etherpad file is rejected
// before it is written.
var ErrInvalidEtherpadFile = errors.New("invalid etherpad file")

// parsedEtherpad is a .etherpad import, parsed and remapped to the target
// pad but not yet written.
type parsedEtherpad struct {
	pad               db2.PadDB
	revisions         map[int]Revision
	authors           map[string]GlobalAuthor
	chats             []ChatMessage
	referencedAuthors []string
}

// validateImport writes imp into an in-memory store and runs the pad
// consistency check on the result.
func (i *Importer) validateImport(padId string, imp *parsedEtherpad) error {
	scratch := db.NewMemoryDataStore()
	if err := i.writeImport(scratch, imp, false); err != nil {
		return err
	}
	// A hook instance without listeners: plugins must not see the probe load.
	noHooks := hooks.NewHook()
	probe := padModel.NewPad(padId, scratch, &noHooks)
	if err := probe.Init(nil, nil, author.NewManager(scratch)); err != nil {
		return err
	}
	return probe.Check()
}

// writeImport stores imp in store. Every write error is returned: an import
// that silently drops a revision leaves a pad that can't be loaded.
func (i *Importer) writeImport(store db.DataStore, imp *parsedEtherpad, live bool) error {
	padId := imp.pad.ID

	known := make(map[string]bool)
	for authorIdFromFile, authorData := range imp.authors {
		known[authorIdFromFile] = true
		colorId := sanitizeImportedColorId(authorData.ColorId)
		existing, _ := store.GetAuthor(authorIdFromFile)
		if existing == nil {
			if err := store.SaveAuthor(db2.AuthorDB{
				ID:        authorIdFromFile,
				Name:      authorData.Name,
				ColorId:   colorId,
				Timestamp: authorData.Timestamp,
			}); err != nil {
				return fmt.Errorf("saving author %s: %w", authorIdFromFile, err)
			}
			continue
		}
		// An existing author keeps their name and colour: whoever can
		// import into one pad must not rename authors of the whole
		// instance (etherpad-lite does not touch existing authors either).
	}
	// Revisions and chat reference their authors; files that lack an author
	// record (hand-made or trimmed exports) get a nameless placeholder so the
	// rows can be stored (SQL datastores enforce the reference).
	for _, id := range imp.referencedAuthors {
		if known[id] {
			continue
		}
		known[id] = true
		if existing, _ := store.GetAuthor(id); existing != nil {
			continue
		}
		if err := store.SaveAuthor(db2.AuthorDB{ID: id, ColorId: "#808080", Timestamp: time.Now().UnixMilli()}); err != nil {
			return fmt.Errorf("saving placeholder author %s: %w", id, err)
		}
	}

	if err := store.CreatePad(padId, imp.pad); err != nil {
		return fmt.Errorf("saving pad: %w", err)
	}

	revNums := make([]int, 0, len(imp.revisions))
	for n := range imp.revisions {
		revNums = append(revNums, n)
	}
	sort.Ints(revNums)
	for _, revNum := range revNums {
		rev := imp.revisions[revNum]
		var timestamp int64
		if rev.Meta.Timestamp != nil {
			timestamp = *rev.Meta.Timestamp
		}
		var revAText db2.AText
		if rev.Meta.AText != nil {
			revAText = db2.AText{Text: rev.Meta.AText.Text, Attribs: rev.Meta.AText.Attribs}
		}
		var revPool db2.RevPool
		if rev.Meta.Pool != nil {
			revPool = db2.RevPool{
				NumToAttrib: rev.Meta.Pool.NumToAttrib,
				AttribToNum: rev.Meta.Pool.AttribToNum,
				NextNum:     rev.Meta.Pool.NextNum,
			}
		}
		// Old files store "" for an unattributed revision.
		revAuthor := rev.Meta.Author
		if revAuthor != nil && *revAuthor == "" {
			revAuthor = nil
		}
		if err := store.SaveRevision(padId, revNum, rev.Changeset, revAText, revPool, revAuthor, timestamp); err != nil {
			return fmt.Errorf("saving revision %d: %w", revNum, err)
		}
	}

	for idx, chat := range imp.chats {
		var ts int64
		if chat.Time != nil {
			ts = *chat.Time
		}
		var userId *string
		if chat.UserId != nil && *chat.UserId != "" {
			userId = chat.UserId
		}
		if err := store.SaveChatMessage(padId, idx, userId, ts, chat.Text); err != nil {
			return fmt.Errorf("saving chat message %d: %w", idx, err)
		}
	}
	if len(imp.chats) > 0 {
		if err := store.SaveChatHeadOfPad(padId, len(imp.chats)-1); err != nil {
			return fmt.Errorf("saving chat head: %w", err)
		}
	}
	return nil
}

// SetPadHTML imports HTML content into a pad
// Note: This currently imports only the text content. Full formatting support
// would require complex changeset generation which is error-prone.
// For full formatting preservation, export/import via .etherpad format is recommended.
func (i *Importer) SetPadHTML(pad *padModel.Pad, htmlContent string, authorId string) error {
	// Parse HTML and extract text
	text, err := i.htmlToText(htmlContent)
	if err != nil {
		return err
	}

	// If no content, just set empty
	if text == "" || text == "\n" {
		return pad.SetText("\n", &authorId)
	}

	// Set the plain text
	return pad.SetText(text, &authorId)
}

// applyAttributeToRange applies an attribute to a range of text in the pad
func (i *Importer) applyAttributeToRange(pad *padModel.Pad, start, length int, attribStr string, authorId *string) error {
	if length <= 0 {
		return nil
	}

	text := pad.Text()
	textLen := len([]rune(text))

	if start >= textLen || start+length > textLen {
		return nil
	}

	// Build a changeset that keeps text before, applies attribute to range, keeps rest
	var ops strings.Builder

	// Keep before (if any)
	if start > 0 {
		beforeText := string([]rune(text)[:start])
		lines := strings.Count(beforeText, "\n")
		if lines > 0 {
			ops.WriteString("|")
			ops.WriteString(strconv.Itoa(lines))
		}
		ops.WriteString("=")
		ops.WriteString(strconv.Itoa(start))
	}

	// Apply attribute to the range
	rangeText := string([]rune(text)[start : start+length])
	lines := strings.Count(rangeText, "\n")
	ops.WriteString(attribStr)
	if lines > 0 {
		ops.WriteString("|")
		ops.WriteString(strconv.Itoa(lines))
	}
	ops.WriteString("=")
	ops.WriteString(strconv.Itoa(length))

	// Build the changeset
	cs := "Z:" + strconv.FormatInt(int64(textLen), 36) + ">0" + ops.String() + "$"

	// Apply the changeset
	_, err := pad.AppendRevision(cs, authorId)
	return err
}

// htmlParseResult contains the parsed HTML content
type htmlParseResult struct {
	text     string
	segments []htmlSegment
}

// htmlSegment represents a segment of text with attributes
type htmlSegment struct {
	start      int
	length     int
	attributes map[string]string
}

// parseHTMLWithFormatting parses HTML and extracts text with formatting information
func (i *Importer) parseHTMLWithFormatting(htmlContent string) (*htmlParseResult, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return nil, err
	}

	result := &htmlParseResult{
		segments: make([]htmlSegment, 0),
	}

	var sb strings.Builder
	activeAttrs := make(map[string]string)
	i.extractHTMLWithFormatting(doc, &sb, result, activeAttrs, 0)

	result.text = sb.String()

	// Clean up multiple newlines
	// We need to adjust segment positions if we clean up the text
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result.text = multipleNewlines.ReplaceAllString(result.text, "\n\n")

	// Ensure text ends with newline
	if !strings.HasSuffix(result.text, "\n") {
		result.text += "\n"
	}

	return result, nil
}

// extractHTMLWithFormatting recursively extracts text and formatting from HTML nodes
func (i *Importer) extractHTMLWithFormatting(n *html.Node, sb *strings.Builder, result *htmlParseResult, activeAttrs map[string]string, depth int) {
	// Skip certain elements
	if n.Type == html.ElementNode {
		switch n.Data {
		case "script", "style", "head":
			return
		}
	}

	// Handle text nodes
	if n.Type == html.TextNode {
		text := n.Data
		// Don't trim space inside inline elements, but clean up excessive whitespace
		text = regexp.MustCompile(`[\t\r]+`).ReplaceAllString(text, " ")

		if text != "" && text != " " {
			startPos := sb.Len()
			// Count runes for proper position
			startRunes := len([]rune(sb.String()))
			sb.WriteString(text)

			// If there are active attributes, create a segment
			if len(activeAttrs) > 0 && len(text) > 0 {
				// Copy attributes
				attrs := make(map[string]string)
				for k, v := range activeAttrs {
					attrs[k] = v
				}
				result.segments = append(result.segments, htmlSegment{
					start:      startRunes,
					length:     len([]rune(text)),
					attributes: attrs,
				})
			}
			_ = startPos // unused but kept for clarity
		}
		return
	}

	// Handle element nodes - determine formatting
	newAttrs := make(map[string]string)
	for k, v := range activeAttrs {
		newAttrs[k] = v
	}

	listType := ""
	isListItem := false
	isBlock := false

	if n.Type == html.ElementNode {
		switch n.Data {
		case "strong", "b":
			newAttrs["bold"] = "true"
		case "em", "i":
			newAttrs["italic"] = "true"
		case "u":
			newAttrs["underline"] = "true"
		case "s", "strike", "del":
			newAttrs["strikethrough"] = "true"
		case "h1":
			newAttrs["heading"] = "h1"
			isBlock = true
		case "h2":
			newAttrs["heading"] = "h2"
			isBlock = true
		case "h3", "h4", "h5", "h6":
			newAttrs["heading"] = "h2" // Map all to h2 for simplicity
			isBlock = true
		case "ul":
			// Check for class to determine list type
			listType = "bullet"
			for _, attr := range n.Attr {
				if attr.Key == "class" {
					if strings.Contains(attr.Val, "bullet") {
						listType = "bullet"
					} else if strings.Contains(attr.Val, "indent") {
						listType = "indent"
					}
				}
			}
		case "ol":
			listType = "number"
		case "li":
			isListItem = true
			isBlock = true
		case "p", "div":
			isBlock = true
		case "br":
			sb.WriteString("\n")
			return
		}
	}

	// Handle list items - add list marker
	if isListItem {
		// Find parent list type
		parent := n.Parent
		for parent != nil {
			if parent.Type == html.ElementNode {
				if parent.Data == "ul" {
					for _, attr := range parent.Attr {
						if attr.Key == "class" {
							if strings.Contains(attr.Val, "bullet") {
								newAttrs["list"] = "bullet1"
							} else if strings.Contains(attr.Val, "indent") {
								newAttrs["list"] = "indent1"
							} else {
								newAttrs["list"] = "bullet1"
							}
							break
						}
					}
					if _, ok := newAttrs["list"]; !ok {
						newAttrs["list"] = "bullet1"
					}
					break
				} else if parent.Data == "ol" {
					newAttrs["list"] = "number1"
					break
				}
			}
			parent = parent.Parent
		}

		// Add list marker character
		sb.WriteString("*")
	}

	// Process children
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		i.extractHTMLWithFormatting(c, sb, result, newAttrs, depth+1)
	}

	// Add newline after block elements
	if isBlock {
		// Only add newline if we don't already end with one
		str := sb.String()
		if len(str) > 0 && !strings.HasSuffix(str, "\n") {
			sb.WriteString("\n")
		}
	}

	// Inherit list type for nested processing
	_ = listType
}

// htmlToText converts HTML to plain text, preserving basic structure (legacy method)
func (i *Importer) htmlToText(htmlContent string) (string, error) {
	doc, err := html.Parse(strings.NewReader(htmlContent))
	if err != nil {
		return "", err
	}

	var sb strings.Builder
	i.extractText(doc, &sb)

	text := sb.String()

	// Clean up multiple newlines
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	text = multipleNewlines.ReplaceAllString(text, "\n\n")

	// Ensure text ends with newline
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}

	return text, nil
}

// extractText recursively extracts text from HTML nodes
func (i *Importer) extractText(n *html.Node, sb *strings.Builder) {
	i.extractTextWithContext(n, sb, false, 0)
}

// extractTextWithContext recursively extracts text from HTML nodes with list context
func (i *Importer) extractTextWithContext(n *html.Node, sb *strings.Builder, inList bool, listCounter int) int {
	if n.Type == html.TextNode {
		text := n.Data
		// Clean up whitespace but preserve single spaces
		text = regexp.MustCompile(`[\t\r\n]+`).ReplaceAllString(text, " ")
		text = strings.TrimSpace(text)
		if text != "" {
			sb.WriteString(text)
		}
		return listCounter
	}

	// Handle block elements - add newlines
	isBlock := false
	isListItem := false
	isOrderedList := false
	isUnorderedList := false

	if n.Type == html.ElementNode {
		switch n.Data {
		case "p", "div", "h1", "h2", "h3", "h4", "h5", "h6", "tr":
			isBlock = true
		case "br":
			sb.WriteString("\n")
			return listCounter
		case "li":
			isListItem = true
			isBlock = true
		case "ol":
			isOrderedList = true
		case "ul":
			isUnorderedList = true
		case "script", "style", "head":
			// Skip these elements entirely
			return listCounter
		}
	}

	// Add list marker before list item content
	if isListItem {
		// Find parent list type
		parent := n.Parent
		for parent != nil {
			if parent.Type == html.ElementNode {
				if parent.Data == "ol" {
					listCounter++
					sb.WriteString(strconv.Itoa(listCounter))
					sb.WriteString(". ")
					break
				} else if parent.Data == "ul" {
					sb.WriteString("• ")
					break
				}
			}
			parent = parent.Parent
		}
	}

	// Reset counter for new ordered list
	if isOrderedList {
		listCounter = 0
	}

	for c := n.FirstChild; c != nil; c = c.NextSibling {
		listCounter = i.extractTextWithContext(c, sb, inList || isUnorderedList || isOrderedList, listCounter)
	}

	if isBlock {
		// Only add newline if we don't already end with one
		str := sb.String()
		if len(str) > 0 && !strings.HasSuffix(str, "\n") {
			sb.WriteString("\n")
		}
	}

	return listCounter
}

// SetPadText imports plain text into a pad
func (i *Importer) SetPadText(pad *padModel.Pad, text string, authorId string) error {
	// Ensure text ends with newline
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}

	return pad.SetText(text, &authorId)
}

// ExtractTextFromDocx extracts text content from a DOCX file
func (i *Importer) ExtractTextFromDocx(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", errors.New("invalid DOCX file: " + err.Error())
	}

	// Find and read the document.xml file
	var documentXML []byte
	for _, file := range reader.File {
		if file.Name == "word/document.xml" {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			documentXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}

	if documentXML == nil {
		return "", errors.New("no document.xml found in DOCX")
	}

	// Parse the XML and extract text
	return i.extractTextFromDocxXML(documentXML)
}

// extractTextFromDocxXML extracts text from DOCX XML content
func (i *Importer) extractTextFromDocxXML(xmlContent []byte) (string, error) {
	var sb strings.Builder
	decoder := xml.NewDecoder(bytes.NewReader(xmlContent))

	inParagraph := false
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// w:p is paragraph, w:br is break
			if t.Name.Local == "p" {
				inParagraph = true
			}
		case xml.EndElement:
			if t.Name.Local == "p" && inParagraph {
				sb.WriteString("\n")
				inParagraph = false
			}
		case xml.CharData:
			text := strings.TrimSpace(string(t))
			if text != "" {
				sb.WriteString(text)
			}
		}
	}

	result := sb.String()
	// Clean up multiple newlines
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result = multipleNewlines.ReplaceAllString(result, "\n\n")

	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result, nil
}

// ExtractTextFromOdt extracts text content from an ODT file
func (i *Importer) ExtractTextFromOdt(content []byte) (string, error) {
	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return "", errors.New("invalid ODT file: " + err.Error())
	}

	// Find and read the content.xml file
	var contentXML []byte
	for _, file := range reader.File {
		if file.Name == "content.xml" {
			rc, err := file.Open()
			if err != nil {
				return "", err
			}
			contentXML, err = io.ReadAll(rc)
			rc.Close()
			if err != nil {
				return "", err
			}
			break
		}
	}

	if contentXML == nil {
		return "", errors.New("no content.xml found in ODT")
	}

	// Parse the XML and extract text
	return i.extractTextFromOdtXML(contentXML)
}

// extractTextFromOdtXML extracts text from ODT XML content
func (i *Importer) extractTextFromOdtXML(xmlContent []byte) (string, error) {
	var sb strings.Builder
	decoder := xml.NewDecoder(bytes.NewReader(xmlContent))

	inParagraph := false
	for {
		token, err := decoder.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		switch t := token.(type) {
		case xml.StartElement:
			// text:p is paragraph, text:h is heading, text:line-break is break
			if t.Name.Local == "p" || t.Name.Local == "h" {
				inParagraph = true
			} else if t.Name.Local == "line-break" {
				sb.WriteString("\n")
			} else if t.Name.Local == "tab" {
				sb.WriteString("\t")
			} else if t.Name.Local == "s" {
				// Space element - check for count attribute
				count := 1
				for _, attr := range t.Attr {
					if attr.Name.Local == "c" {
						if n, err := strconv.Atoi(attr.Value); err == nil {
							count = n
						}
					}
				}
				sb.WriteString(strings.Repeat(" ", count))
			}
		case xml.EndElement:
			if (t.Name.Local == "p" || t.Name.Local == "h") && inParagraph {
				sb.WriteString("\n")
				inParagraph = false
			}
		case xml.CharData:
			text := string(t)
			if text != "" && inParagraph {
				sb.WriteString(text)
			}
		}
	}

	result := sb.String()
	// Clean up multiple newlines
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result = multipleNewlines.ReplaceAllString(result, "\n\n")

	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result, nil
}

// ExtractTextFromRtf extracts text content from an RTF file
func (i *Importer) ExtractTextFromRtf(content []byte) (string, error) {
	// RTF parsing is complex, we use a simplified approach
	// that strips RTF control words and extracts plain text
	text := string(content)

	// Remove RTF header
	if !strings.HasPrefix(text, "{\\rtf") {
		return "", errors.New("invalid RTF file")
	}

	var sb strings.Builder
	inGroup := 0
	skipGroup := false
	i2 := 0

	for i2 < len(text) {
		ch := text[i2]

		if ch == '{' {
			inGroup++
			// Check if this is a group to skip (like \fonttbl, \colortbl, etc.)
			if i2+1 < len(text) && text[i2+1] == '\\' {
				rest := text[i2+1:]
				if strings.HasPrefix(rest, "\\fonttbl") ||
					strings.HasPrefix(rest, "\\colortbl") ||
					strings.HasPrefix(rest, "\\stylesheet") ||
					strings.HasPrefix(rest, "\\info") ||
					strings.HasPrefix(rest, "\\*") {
					skipGroup = true
				}
			}
			i2++
			continue
		}

		if ch == '}' {
			inGroup--
			if inGroup == 0 {
				skipGroup = false
			}
			i2++
			continue
		}

		if skipGroup {
			i2++
			continue
		}

		if ch == '\\' {
			// Control word
			i2++
			if i2 >= len(text) {
				break
			}

			// Special characters
			if text[i2] == '\'' && i2+2 < len(text) {
				// Hex character
				i2 += 3
				continue
			}

			// Check for special control words
			if strings.HasPrefix(text[i2:], "par") || strings.HasPrefix(text[i2:], "line") {
				sb.WriteString("\n")
			} else if strings.HasPrefix(text[i2:], "tab") {
				sb.WriteString("\t")
			}

			// Skip the control word
			for i2 < len(text) && ((text[i2] >= 'a' && text[i2] <= 'z') || (text[i2] >= 'A' && text[i2] <= 'Z')) {
				i2++
			}
			// Skip optional numeric parameter
			for i2 < len(text) && ((text[i2] >= '0' && text[i2] <= '9') || text[i2] == '-') {
				i2++
			}
			// Skip optional space after control word
			if i2 < len(text) && text[i2] == ' ' {
				i2++
			}
			continue
		}

		if ch == '\n' || ch == '\r' {
			i2++
			continue
		}

		// Regular character
		sb.WriteByte(ch)
		i2++
	}

	result := sb.String()
	// Clean up multiple newlines
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result = multipleNewlines.ReplaceAllString(result, "\n\n")

	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result, nil
}

// ExtractTextFromPdf extracts text content from a PDF file using the ledongthuc/pdf library
func (i *Importer) ExtractTextFromPdf(content []byte) (string, error) {
	// Check for PDF header
	if len(content) < 4 || string(content[:4]) != "%PDF" {
		return "", errors.New("invalid PDF file")
	}

	// Create a reader from the content
	reader := bytes.NewReader(content)

	// Open the PDF
	pdfReader, err := pdf.NewReader(reader, int64(len(content)))
	if err != nil {
		return "", fmt.Errorf("could not open PDF: %w", err)
	}

	var sb strings.Builder

	// Extract text from all pages
	numPages := pdfReader.NumPage()
	for pageNum := 1; pageNum <= numPages; pageNum++ {
		page := pdfReader.Page(pageNum)
		if page.V.IsNull() {
			continue
		}

		text, err := page.GetPlainText(nil)
		if err != nil {
			// Try to continue with other pages even if one fails
			i.logger.Warnf("Could not extract text from page %d: %v", pageNum, err)
			continue
		}

		if text != "" {
			sb.WriteString(text)
			// Add page separator if not the last page
			if pageNum < numPages {
				sb.WriteString("\n")
			}
		}
	}

	result := sb.String()

	// If no text was extracted, return an error
	if strings.TrimSpace(result) == "" {
		return "", errors.New("could not extract text from PDF - the PDF may be image-based or encrypted")
	}

	// Clean up multiple newlines
	multipleNewlines := regexp.MustCompile(`\n{3,}`)
	result = multipleNewlines.ReplaceAllString(result, "\n\n")

	if !strings.HasSuffix(result, "\n") {
		result += "\n"
	}

	return result, nil
}

// ExtractEtherpadFromPdf extracts embedded Etherpad JSON data from a PDF file
// This is similar to how ZUGFeRD embeds XML in PDF, but uses JSON for Etherpad data
// Returns nil if no embedded Etherpad data is found
func (i *Importer) ExtractEtherpadFromPdf(content []byte) ([]byte, error) {
	// Check for PDF header
	if len(content) < 4 || string(content[:4]) != "%PDF" {
		return nil, errors.New("invalid PDF file")
	}

	// Create temporary directory for pdfcpu operations
	tempDir, err := os.MkdirTemp("", "etherpad-pdf-import-*")
	if err != nil {
		return nil, fmt.Errorf("could not create temp dir: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Write PDF to temp file
	inputPdfPath := tempDir + "/input.pdf"
	if err := os.WriteFile(inputPdfPath, content, 0644); err != nil {
		return nil, fmt.Errorf("could not write pdf temp file: %w", err)
	}

	// Configure pdfcpu for relaxed validation
	conf := model.NewDefaultConfiguration()
	conf.ValidationMode = model.ValidationRelaxed

	// Extract attachments from the PDF
	outputDir := tempDir + "/attachments"
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, fmt.Errorf("could not create output dir: %w", err)
	}

	ctx := context.Background()
	err = api.ExtractAttachmentsFile(ctx, inputPdfPath, outputDir, []string{"etherpad.json"}, conf)
	if err != nil {
		// Try extracting all attachments if specific one fails
		err = api.ExtractAttachmentsFile(ctx, inputPdfPath, outputDir, nil, conf)
		if err != nil {
			return nil, fmt.Errorf("could not extract attachments: %w", err)
		}
	}

	// Look for etherpad.json in the output directory
	jsonPath := outputDir + "/etherpad.json"
	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		return nil, errors.New("no embedded Etherpad data found in PDF")
	}

	i.logger.Info("Found embedded etherpad.json in PDF")
	return jsonData, nil
}

// ExtractTextFromEtherpadJson extracts the plain text content from Etherpad JSON export
func (i *Importer) ExtractTextFromEtherpadJson(content []byte) (string, error) {
	var rawData map[string]json.RawMessage
	if err := json.Unmarshal(content, &rawData); err != nil {
		return "", fmt.Errorf("could not parse etherpad JSON: %w", err)
	}

	padRegex := regexp.MustCompile(`^pad:.+:$`)
	for key, value := range rawData {
		if padRegex.MatchString(key) {
			var padData PadData
			if err := json.Unmarshal(value, &padData); err == nil && padData.AText.Text != "" {
				if i.logger != nil {
					i.logger.Infof("Extracted text from Etherpad JSON (key: %s, length: %d)", key, len(padData.AText.Text))
				}
				return padData.AText.Text, nil
			}
		}
	}

	return "", errors.New("no text content found in etherpad JSON")
}
