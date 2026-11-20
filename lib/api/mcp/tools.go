package mcp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/ether/etherpad-go/lib/accounts"
	"github.com/ether/etherpad-go/lib/db"
	pad "github.com/ether/etherpad-go/lib/models/pad"
	"github.com/ether/etherpad-go/lib/models/ws"
	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// maxText bounds the text of one call, as a new pad's text is bounded.
	maxText = 100000
	// maxReplacements bounds replace_text with all: each is a revision.
	maxReplacements = 100
	maxListedPads   = 500
	maxChat         = 200
)

func yes() *bool { b := true; return &b }
func no() *bool  { b := false; return &b }

func (s *Server) addTools() {
	sdk.AddTool(s.mcp, &sdk.Tool{Name: "list_pads", Title: "List pads",
		Description: "Lists the text pads this token may read, by id, in order. query narrows them to ids that contain it; " +
			"when nextCursor is returned, pass it as cursor for the next page.",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: no()}}, s.listPads)
	sdk.AddTool(s.mcp, &sdk.Tool{Name: "read_pad", Title: "Read a pad",
		Description: "Returns the text of a pad and its revision.",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: no()}}, s.readPad)
	sdk.AddTool(s.mcp, &sdk.Tool{Name: "create_pad", Title: "Create a pad",
		Description: "Creates a pad with the given text (the instance's welcome text when empty). Fails if it exists.",
		Annotations: &sdk.ToolAnnotations{DestructiveHint: no(), OpenWorldHint: no()}}, s.createPad)
	sdk.AddTool(s.mcp, &sdk.Tool{Name: "append_text", Title: "Append text",
		Description: "Adds text at the end of a pad, right after its last character: start it with a newline for a new line. " +
			"The rest of the pad is left as it is.",
		Annotations: &sdk.ToolAnnotations{DestructiveHint: no(), OpenWorldHint: no()}}, s.appendText)
	sdk.AddTool(s.mcp, &sdk.Tool{Name: "replace_text", Title: "Replace text",
		Description: "Replaces text in a pad: find must occur exactly once, or set all to replace every occurrence (at most 100). " +
			"Only the replaced text changes; formatting and authorship of the rest stay. An empty replace deletes.",
		Annotations: &sdk.ToolAnnotations{DestructiveHint: yes(), OpenWorldHint: no()}}, s.replaceText)
	sdk.AddTool(s.mcp, &sdk.Tool{Name: "set_text", Title: "Replace the whole pad",
		Description: "Replaces the whole text of a pad. Its formatting and who wrote what are lost for the new revision " +
			"(the history keeps them); prefer replace_text or append_text.",
		Annotations: &sdk.ToolAnnotations{DestructiveHint: yes(), IdempotentHint: true, OpenWorldHint: no()}}, s.setText)
	sdk.AddTool(s.mcp, &sdk.Tool{Name: "read_chat", Title: "Read the chat",
		Description: "Returns the last messages of a pad's chat, oldest first.",
		Annotations: &sdk.ToolAnnotations{ReadOnlyHint: true, OpenWorldHint: no()}}, s.readChat)
	sdk.AddTool(s.mcp, &sdk.Tool{Name: "send_chat", Title: "Write in the chat",
		Description: "Writes a message in a pad's chat; everyone in the pad sees it at once.",
		Annotations: &sdk.ToolAnnotations{DestructiveHint: no(), OpenWorldHint: no()}}, s.sendChat)
}

// ── list_pads ──

type listPadsIn struct {
	Query  string `json:"query,omitempty" jsonschema:"only pads whose id contains this text"`
	Limit  int    `json:"limit,omitempty" jsonschema:"most pads to return (default 100, at most 500)"`
	Cursor string `json:"cursor,omitempty" jsonschema:"nextCursor of the call before, for the next page"`
}

type listedPad struct {
	ID       string `json:"id"`
	Writable bool   `json:"writable"`
}

type listPadsOut struct {
	Pads []listedPad `json:"pads"`
	// NextCursor: there may be more; pass it as cursor for the next page.
	NextCursor string `json:"nextCursor,omitempty"`
}

const (
	// listBatch is how many ids one database query reads.
	listBatch = 500
	// maxScanned bounds the ids one call looks at: a token that may read
	// few pads of many gets a cursor instead of a scan of everything.
	maxScanned = 5000
)

// literalPrefix is the part of a pad pattern before its first "*".
func literalPrefix(pattern string) string {
	if i := strings.IndexByte(pattern, '*'); i >= 0 {
		return pattern[:i]
	}
	return pattern
}

func (s *Server) listPads(_ context.Context, req *sdk.CallToolRequest, in listPadsIn) (*sdk.CallToolResult, listPadsOut, error) {
	out := listPadsOut{Pads: []listedPad{}}
	use, err := useOf(req)
	if err != nil {
		return nil, out, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 100
	}
	limit = min(limit, maxListedPads)
	// The token's pattern narrows the query in the database; the rights
	// of its account are checked per pad below.
	prefix := literalPrefix(use.Token.PadPattern)
	after := in.Cursor
	for scanned := 0; scanned < maxScanned; {
		ids, err := s.store.Store.ListPadIDs(db.PadListing{After: after, Prefix: prefix, Contains: strings.TrimSpace(in.Query),
			DocumentType: "text", Limit: listBatch})
		if err != nil {
			return nil, out, err
		}
		for _, id := range ids {
			after = id
			scanned++
			if !use.Grants.Has(accounts.ScopePadRead, id) {
				continue
			}
			out.Pads = append(out.Pads, listedPad{ID: id, Writable: use.Grants.Has(accounts.ScopePadWrite, id)})
			if len(out.Pads) == limit {
				out.NextCursor = id
				return nil, out, nil
			}
		}
		if len(ids) < listBatch {
			return nil, out, nil
		}
	}
	out.NextCursor = after
	return nil, out, nil
}

// ── read_pad ──

type padIn struct {
	Pad string `json:"pad" jsonschema:"the pad's id, as in its address /p/<id>"`
}

type readPadOut struct {
	Pad      string `json:"pad"`
	Text     string `json:"text"`
	Revision int    `json:"revision"`
}

func (s *Server) readPad(_ context.Context, req *sdk.CallToolRequest, in padIn) (*sdk.CallToolResult, readPadOut, error) {
	var out readPadOut
	use, err := useOf(req)
	if err != nil {
		return nil, out, err
	}
	id, err := s.padID(use, in.Pad, accounts.ScopePadRead)
	if err != nil {
		return nil, out, err
	}
	unlock, err := s.store.PadManager.RLockPad(id)
	if err != nil {
		return nil, out, err
	}
	defer unlock()
	p, err := s.existing(id)
	if err != nil {
		return nil, out, err
	}
	return nil, readPadOut{Pad: id, Text: p.Text(), Revision: p.Head}, nil
}

// existing returns a text pad that exists; the caller holds its lock.
func (s *Server) existing(id string) (*pad.Pad, error) {
	exists, err := s.store.PadManager.DoesPadExist(id)
	if err != nil {
		return nil, err
	}
	if !*exists {
		return nil, fmt.Errorf("pad %q does not exist", id)
	}
	p, err := s.store.PadManager.GetPad(id, nil, nil)
	if err != nil {
		return nil, err
	}
	if p.DocumentType != "" && p.DocumentType != "text" {
		return nil, fmt.Errorf("pad %q is a %s, not a text pad; these tools do not handle it yet", id, p.DocumentType)
	}
	return p, nil
}

// ── changes ──

type changeOut struct {
	Pad      string `json:"pad"`
	Revision int    `json:"revision"`
	// Replaced counts the replacements of replace_text.
	Replaced int `json:"replaced,omitempty"`
}

func checkText(name, text string) error {
	if utf8.RuneCountInString(text) > maxText {
		return fmt.Errorf("%s is longer than %d characters", name, maxText)
	}
	return nil
}

// change runs fn on an existing text pad under its lock, as the token's
// author, and sends the new revision to the pad's clients.
func (s *Server) change(req *sdk.CallToolRequest, raw string, fn func(p *pad.Pad, author string) (int, error)) (changeOut, error) {
	var out changeOut
	use, err := useOf(req)
	if err != nil {
		return out, err
	}
	id, err := s.padID(use, raw, accounts.ScopePadWrite)
	if err != nil {
		return out, err
	}
	author, err := s.authorOf(use)
	if err != nil {
		return out, err
	}
	unlock, err := s.store.PadManager.LockPad(id)
	if err != nil {
		return out, err
	}
	defer unlock()
	p, err := s.existing(id)
	if err != nil {
		return out, err
	}
	head := p.Head
	n, err := fn(p, author)
	if p.Head != head {
		// Also after a later step failed: what was written is sent.
		s.store.Handler.UpdatePadClients(p)
	}
	if err != nil {
		return out, err
	}
	return changeOut{Pad: id, Revision: p.Head, Replaced: n}, nil
}

type appendIn struct {
	Pad  string `json:"pad" jsonschema:"the pad's id"`
	Text string `json:"text" jsonschema:"the text to add; start it with a newline for a new line"`
}

func (s *Server) appendText(_ context.Context, req *sdk.CallToolRequest, in appendIn) (*sdk.CallToolResult, changeOut, error) {
	if in.Text == "" {
		return nil, changeOut{}, errors.New("text is required")
	}
	if err := checkText("text", in.Text); err != nil {
		return nil, changeOut{}, err
	}
	out, err := s.change(req, in.Pad, func(p *pad.Pad, author string) (int, error) {
		// Before the pad's final newline, which every pad ends with.
		end := utf8.RuneCountInString(p.Text()) - 1
		return 0, p.SpliceText(max(end, 0), 0, in.Text, &author)
	})
	return nil, out, err
}

type replaceIn struct {
	Pad     string `json:"pad" jsonschema:"the pad's id"`
	Find    string `json:"find" jsonschema:"the exact text to replace"`
	Replace string `json:"replace" jsonschema:"the new text; empty deletes"`
	All     bool   `json:"all,omitempty" jsonschema:"replace every occurrence instead of exactly one"`
}

// occurrences are the rune offsets of find in text, not overlapping.
func occurrences(text, find string) []int {
	var out []int
	offset, runes := 0, 0
	for {
		i := strings.Index(text[offset:], find)
		if i < 0 {
			return out
		}
		runes += utf8.RuneCountInString(text[offset : offset+i])
		out = append(out, runes)
		runes += utf8.RuneCountInString(find)
		offset += i + len(find)
	}
}

func (s *Server) replaceText(_ context.Context, req *sdk.CallToolRequest, in replaceIn) (*sdk.CallToolResult, changeOut, error) {
	if in.Find == "" {
		return nil, changeOut{}, errors.New("find is required")
	}
	if err := checkText("replace", in.Replace); err != nil {
		return nil, changeOut{}, err
	}
	out, err := s.change(req, in.Pad, func(p *pad.Pad, author string) (int, error) {
		text := p.Text()
		// The final newline belongs to the pad, not to its text.
		body := strings.TrimSuffix(text, "\n")
		at := occurrences(body, in.Find)
		switch {
		case len(at) == 0:
			return 0, errors.New("find does not occur in the pad; read it again, it may have changed")
		case len(at) > 1 && !in.All:
			return 0, fmt.Errorf("find occurs %d times; make it longer so it occurs once, or set all", len(at))
		case len(at) > maxReplacements:
			return 0, fmt.Errorf("find occurs %d times, more than %d", len(at), maxReplacements)
		}
		n := utf8.RuneCountInString(in.Find)
		// From the last one back, so the offsets before stay right.
		for i := len(at) - 1; i >= 0; i-- {
			if err := p.SpliceText(at[i], n, in.Replace, &author); err != nil {
				return len(at) - 1 - i, err
			}
		}
		return len(at), nil
	})
	return nil, out, err
}

type setIn struct {
	Pad  string `json:"pad" jsonschema:"the pad's id"`
	Text string `json:"text" jsonschema:"the pad's new text"`
}

func (s *Server) setText(_ context.Context, req *sdk.CallToolRequest, in setIn) (*sdk.CallToolResult, changeOut, error) {
	if err := checkText("text", in.Text); err != nil {
		return nil, changeOut{}, err
	}
	out, err := s.change(req, in.Pad, func(p *pad.Pad, author string) (int, error) {
		if strings.TrimSuffix(p.Text(), "\n") == strings.TrimSuffix(in.Text, "\n") {
			return 0, nil
		}
		return 0, p.SetText(in.Text, &author)
	})
	return nil, out, err
}

type createIn struct {
	Pad  string `json:"pad" jsonschema:"the new pad's id"`
	Text string `json:"text,omitempty" jsonschema:"its text; empty for the instance's welcome text"`
}

func (s *Server) createPad(_ context.Context, req *sdk.CallToolRequest, in createIn) (*sdk.CallToolResult, changeOut, error) {
	var out changeOut
	use, err := useOf(req)
	if err != nil {
		return nil, out, err
	}
	if err := checkText("text", in.Text); err != nil {
		return nil, out, err
	}
	id, err := s.padID(use, in.Pad, accounts.ScopePadCreate)
	if err != nil {
		return nil, out, err
	}
	// Group pads ("g.…$name") belong to their group: the group API makes
	// them, as in the HTTP API's createPad.
	if strings.Contains(id, "$") {
		return nil, out, errors.New("create_pad cannot create group pads")
	}
	if !s.store.PadManager.IsValidPadId(id) {
		return nil, out, fmt.Errorf("invalid pad id %q", in.Pad)
	}
	author, err := s.authorOf(use)
	if err != nil {
		return nil, out, err
	}
	unlock, err := s.store.PadManager.LockPad(id)
	if err != nil {
		return nil, out, err
	}
	defer unlock()
	if exists, err := s.store.PadManager.DoesPadExist(id); err != nil {
		return nil, out, err
	} else if *exists {
		return nil, out, fmt.Errorf("pad %q already exists", id)
	}
	var text *string
	if in.Text != "" {
		// A new pad gets its final newline added: one given is not doubled.
		t := strings.TrimSuffix(in.Text, "\n")
		text = &t
	}
	p, err := s.store.PadManager.GetPad(id, text, &author)
	if err != nil {
		return nil, out, err
	}
	return nil, changeOut{Pad: id, Revision: p.Head}, nil
}

// ── chat ──

type readChatIn struct {
	Pad   string `json:"pad" jsonschema:"the pad's id"`
	Limit int    `json:"limit,omitempty" jsonschema:"how many of the last messages (default 20, at most 200)"`
}

type chatMessage struct {
	Time   string `json:"time"`
	Author string `json:"author"`
	Text   string `json:"text"`
}

type readChatOut struct {
	Pad      string        `json:"pad"`
	Messages []chatMessage `json:"messages"`
}

func (s *Server) readChat(_ context.Context, req *sdk.CallToolRequest, in readChatIn) (*sdk.CallToolResult, readChatOut, error) {
	out := readChatOut{Messages: []chatMessage{}}
	use, err := useOf(req)
	if err != nil {
		return nil, out, err
	}
	id, err := s.padID(use, in.Pad, accounts.ScopePadRead)
	if err != nil {
		return nil, out, err
	}
	limit := in.Limit
	if limit <= 0 {
		limit = 20
	}
	limit = min(limit, maxChat)
	unlock, err := s.store.PadManager.RLockPad(id)
	if err != nil {
		return nil, out, err
	}
	defer unlock()
	p, err := s.existing(id)
	if err != nil {
		return nil, out, err
	}
	out.Pad = id
	if p.ChatHead < 0 {
		return nil, out, nil
	}
	msgs, err := p.GetChatMessages(max(0, p.ChatHead-limit+1), p.ChatHead)
	if err != nil {
		return nil, out, err
	}
	if msgs != nil {
		for _, m := range *msgs {
			name := ""
			if m.DisplayName != nil {
				name = *m.DisplayName
			}
			out.Messages = append(out.Messages, chatMessage{Time: ms(m.Time), Author: name, Text: m.Message})
		}
	}
	return nil, out, nil
}

type sendChatIn struct {
	Pad  string `json:"pad" jsonschema:"the pad's id"`
	Text string `json:"text" jsonschema:"the message"`
}

type sendChatOut struct {
	Pad string `json:"pad"`
}

// maxChatText bounds a chat message.
const maxChatText = 10000

func (s *Server) sendChat(_ context.Context, req *sdk.CallToolRequest, in sendChatIn) (*sdk.CallToolResult, sendChatOut, error) {
	var out sendChatOut
	use, err := useOf(req)
	if err != nil {
		return nil, out, err
	}
	if strings.TrimSpace(in.Text) == "" {
		return nil, out, errors.New("text is required")
	}
	if utf8.RuneCountInString(in.Text) > maxChatText {
		return nil, out, fmt.Errorf("the message is longer than %d characters", maxChatText)
	}
	id, err := s.padID(use, in.Pad, accounts.ScopeChatWrite)
	if err != nil {
		return nil, out, err
	}
	author, err := s.authorOf(use)
	if err != nil {
		return nil, out, err
	}
	if err := s.mustExist(id); err != nil {
		return nil, out, err
	}
	// AppendChat checks again under the pad's lock that it still exists.
	now := time.Now().UnixMilli()
	if err := s.store.Handler.AppendChat(id, ws.ChatMessageData{Text: in.Text, Time: &now, AuthorId: &author}); err != nil {
		return nil, out, err
	}
	return nil, sendChatOut{Pad: id}, nil
}

// mustExist checks that a text pad exists without loading it into a lock.
func (s *Server) mustExist(id string) error {
	unlock, err := s.store.PadManager.RLockPad(id)
	if err != nil {
		return err
	}
	defer unlock()
	_, err = s.existing(id)
	return err
}
