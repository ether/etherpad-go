package ws

import (
	"encoding/json"

	"github.com/ether/etherpad-go/lib/author"
	"github.com/ether/etherpad-go/lib/settings"
)

// The admin's author list and erasure (etherpad-lite's authorLoad,
// anonymizeAuthorPreview, anonymizeAuthor). All of it needs
// gdprAuthorErasure.enabled: without it the events answer
// {error: "disabled"} and list nobody.

func isAuthorEvent(event string) bool {
	switch event {
	case "authorLoad", "anonymizeAuthorPreview", "anonymizeAuthor":
		return true
	}
	return false
}

type authorLoadRequest struct {
	Pattern       string `json:"pattern"`
	Offset        int    `json:"offset"`
	Limit         int    `json:"limit"`
	SortBy        string `json:"sortBy"`
	Ascending     bool   `json:"ascending"`
	IncludeErased bool   `json:"includeErased"`
}

type authorErasureReply struct {
	AuthorID string `json:"authorID,omitempty"`
	*author.ErasureResult
	Error string `json:"error,omitempty"`
}

func (h AdminMessageHandler) handleAuthorEvent(event string, data json.RawMessage, s *settings.Settings, c *Client) {
	reply := func(v any) {
		if b, err := json.Marshal([]any{"results:" + event, v}); err == nil {
			c.SafeSend(b)
		}
	}
	enabled := s != nil && s.GdprAuthorErasure.Enabled
	manager := author.NewManager(h.store)

	switch event {
	case "authorLoad":
		if !enabled {
			reply(map[string]any{"error": "disabled", "total": 0, "results": []any{}})
			return
		}
		req := authorLoadRequest{Limit: 12, SortBy: "name", Ascending: true}
		if len(data) > 0 {
			_ = json.Unmarshal(data, &req)
		}
		res, err := manager.SearchAuthors(req.Pattern, req.Offset, req.Limit, req.SortBy, req.Ascending, req.IncludeErased)
		if err != nil {
			h.Logger.Errorf("admin: listing authors: %v", err)
			reply(map[string]any{"error": "failed", "total": 0, "results": []any{}})
			return
		}
		reply(res)
	case "anonymizeAuthorPreview", "anonymizeAuthor":
		if !enabled {
			reply(authorErasureReply{Error: "disabled"})
			return
		}
		var req struct {
			AuthorID string `json:"authorID"`
		}
		if len(data) > 0 {
			_ = json.Unmarshal(data, &req)
		}
		if req.AuthorID == "" {
			reply(authorErasureReply{Error: "authorID is required"})
			return
		}
		dryRun := event == "anonymizeAuthorPreview"
		res, err := manager.Anonymize(req.AuthorID, dryRun)
		if err != nil {
			reply(authorErasureReply{AuthorID: req.AuthorID, Error: err.Error()})
			return
		}
		if !dryRun {
			h.Logger.Infof("admin: erased author %s (%d pads, %d chat messages)", req.AuthorID, res.AffectedPads, res.ClearedChatMessages)
		}
		reply(authorErasureReply{AuthorID: req.AuthorID, ErasureResult: &res})
	}
}
