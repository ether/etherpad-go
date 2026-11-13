package ws

import (
	"encoding/json"
	"errors"

	"github.com/ether/etherpad-go/lib/apikeys"
	"github.com/ether/etherpad-go/lib/db"
)

// APIKeyView is a key as the admin page lists it: never the key itself.
type APIKeyView struct {
	ID          string `json:"id"`
	Description string `json:"description"`
	Prefix      string `json:"prefix"`
	CreatedAt   int64  `json:"createdAt"`
	LastUsed    *int64 `json:"lastUsed"`
	Disabled    bool   `json:"disabled"`
}

// APIKeysResult answers every API key event: the keys, and for a created
// key the key itself, which is not shown again.
type APIKeysResult struct {
	Keys []APIKeyView `json:"keys"`
	// Created is set for apiKeyCreate.
	Created *APIKeyCreated `json:"created,omitempty"`
	Error   string         `json:"error,omitempty"`
}

type APIKeyCreated struct {
	ID  string `json:"id"`
	Key string `json:"key"`
}

func isAPIKeyEvent(event string) bool {
	switch event {
	case "apiKeysLoad", "apiKeyCreate", "apiKeySetDisabled", "apiKeyDelete":
		return true
	}
	return false
}

// handleAPIKeyEvent manages the HTTP API keys. Every event is answered
// with "results:apiKeys".
func (h AdminMessageHandler) handleAPIKeyEvent(event string, data json.RawMessage, c *Client) {
	keys := apikeys.NewManager(h.store)
	result := APIKeysResult{}
	var err error
	switch event {
	case "apiKeyCreate":
		var req struct {
			Description string `json:"description"`
		}
		if err = json.Unmarshal(data, &req); err == nil {
			var key string
			var k db.APIKey
			key, k, err = keys.Create(req.Description)
			if err == nil {
				result.Created = &APIKeyCreated{ID: k.ID, Key: key}
				h.Logger.Infof("admin: created HTTP API key %s (%s)", k.Prefix, k.Description)
			}
		}
	case "apiKeySetDisabled":
		var req struct {
			ID       string `json:"id"`
			Disabled bool   `json:"disabled"`
		}
		if err = json.Unmarshal(data, &req); err == nil {
			err = keys.SetDisabled(req.ID, req.Disabled)
			if err == nil {
				h.Logger.Infof("admin: HTTP API key %s disabled=%v", req.ID, req.Disabled)
			}
		}
	case "apiKeyDelete":
		var req struct {
			ID string `json:"id"`
		}
		if err = json.Unmarshal(data, &req); err == nil {
			err = keys.Delete(req.ID)
			if err == nil {
				h.Logger.Infof("admin: deleted HTTP API key %s", req.ID)
			}
		}
	}
	if err != nil {
		switch {
		case errors.Is(err, apikeys.ErrNoDescription), errors.Is(err, apikeys.ErrDescriptionTooLong),
			errors.Is(err, db.ErrAPIKeyNotFound):
			result.Error = err.Error()
		default:
			var syntax *json.SyntaxError
			var typeErr *json.UnmarshalTypeError
			if errors.As(err, &syntax) || errors.As(err, &typeErr) {
				result.Error = "invalid request"
			} else {
				h.Logger.Errorf("admin: API key %s: %v", event, err)
				result.Error = "the API keys could not be changed"
			}
		}
	}
	list, listErr := keys.List()
	if listErr != nil {
		h.Logger.Errorf("admin: listing API keys: %v", listErr)
		if result.Error == "" {
			result.Error = "the API keys could not be loaded"
		}
	}
	result.Keys = make([]APIKeyView, 0, len(list))
	for _, k := range list {
		v := APIKeyView{ID: k.ID, Description: k.Description, Prefix: k.Prefix, CreatedAt: k.CreatedAt.UnixMilli(), Disabled: k.Disabled}
		if k.LastUsed != nil {
			ms := k.LastUsed.UnixMilli()
			v.LastUsed = &ms
		}
		result.Keys = append(result.Keys, v)
	}
	resp, err := json.Marshal([]any{"results:apiKeys", result})
	if err != nil {
		return
	}
	c.SafeSend(resp)
}
