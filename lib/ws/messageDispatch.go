package ws

import (
	"encoding/json"
	"errors"

	"github.com/ether/etherpad-go/lib/models/ws"
	"github.com/ether/etherpad-go/lib/settings"
	"github.com/ether/etherpad-go/lib/ws/ratelimiter"
	"go.uber.org/zap"
)

// Message types the pad socket accepts from clients.
const (
	msgClientReady    = "CLIENT_READY"
	msgCollabRoom     = "COLLABROOM"
	msgPadDelete      = "PAD_DELETE"
	msgSaveRevision   = "SAVE_REVISION"
	msgUserChanges    = "USER_CHANGES"
	msgSheetOp        = "SHEET_OP"
	msgSheetPresence  = "SHEET_PRESENCE"
	msgUserInfoUpdate = "USERINFO_UPDATE"
	msgGetChatMsgs    = "GET_CHAT_MESSAGES"
	msgChangesetReq   = "CHANGESET_REQ"
	msgChatMessage    = "CHAT_MESSAGE"
	msgClientMessage  = "CLIENT_MESSAGE"
)

// incomingEnvelope is the wire format sent by the client's socket wrapper:
//
//	{"event":"message","data":{"type":"CLIENT_READY", ...}}
//	{"event":"message","data":{"type":"COLLABROOM","component":"pad","data":{"type":"USER_CHANGES", ...}}}
//
// Only the routing fields are decoded here; the full message is decoded into
// its concrete struct once the type is known.
type incomingEnvelope struct {
	Event string `json:"event"`
	Data  struct {
		Type string          `json:"type"`
		Data json.RawMessage `json:"data"`
	} `json:"data"`
}

// parseMessageType returns the effective message type and, for COLLABROOM
// messages, the raw inner payload (data.data).
//
// The type is read from the JSON structure, never by searching the raw text:
// pad content, chat text and author names travel inside these messages, so a
// user typing "SAVE_REVISION" into the pad must not change how the message
// carrying that keystroke is routed.
func parseMessageType(message []byte) (string, json.RawMessage, error) {
	var env incomingEnvelope
	if err := json.Unmarshal(message, &env); err != nil {
		return "", nil, err
	}
	if env.Data.Type != msgCollabRoom {
		if env.Data.Type == "" {
			return "", nil, errors.New("message has no type")
		}
		return env.Data.Type, nil, nil
	}
	var inner struct {
		Type string `json:"type"`
	}
	if len(env.Data.Data) == 0 {
		return "", nil, errors.New("COLLABROOM message has no data")
	}
	if err := json.Unmarshal(env.Data.Data, &inner); err != nil {
		return "", nil, err
	}
	if inner.Type == "" {
		return "", nil, errors.New("COLLABROOM message has no inner type")
	}
	return inner.Type, env.Data.Data, nil
}

// decodeInto unmarshals message into a new T, logging failures.
func decodeInto[T any](message []byte, msgType string, logger *zap.SugaredLogger) (T, bool) {
	var v T
	if err := json.Unmarshal(message, &v); err != nil {
		logger.Errorf("Error unmarshalling %s: %v", msgType, err)
		return v, false
	}
	return v, true
}

// dispatchMessage routes a single client message to its handler.
func (c *Client) dispatchMessage(message []byte, retrievedSettings *settings.Settings, logger *zap.SugaredLogger) {
	// A message that makes a handler panic must not take the whole server
	// down. The connection is closed so the client reconnects and resyncs:
	// whatever the panic interrupted was not stored, and the client resends
	// unacknowledged changes after reconnecting.
	defer recoverClientPanic(c, retrievedSettings, logger, "pad")
	msgType, inner, err := parseMessageType(message)
	if err != nil {
		logger.Warnf("Ignoring malformed message from %s: %v", retrievedSettings.LogIP(c.ClientIP), err)
		return
	}

	// CommitRateLimiting only covers commits (USER_CHANGES / SHEET_OP), as
	// in etherpad-lite. Ephemeral traffic like SHEET_PRESENCE arrives per
	// keystroke and must not burn the commit budget — a drained budget
	// silently drops the commit itself and edits are lost.
	if msgType == msgUserChanges || msgType == msgSheetOp {
		retrievedSettings.CommitRateLimiting.LoadTest = retrievedSettings.LoadTest
		if err := ratelimiter.CheckRateLimit(ratelimiter.IPAddress(c.ClientIP), retrievedSettings.CommitRateLimiting); err != nil {
			logger.Warn("Rate limit exceeded:", err.Error())
			return
		}
	}

	switch msgType {
	case msgClientReady:
		// Decode errors were historically only logged here (the handler
		// validates the fields itself), so keep forwarding the message.
		var clientReady ws.ClientReady
		if err := json.Unmarshal(message, &clientReady); err != nil {
			logger.Warn("Error unmarshalling", err)
		}
		c.Handler.HandleMessage(clientReady, c, retrievedSettings, logger)
	case msgPadDelete:
		// PadDelete mirrors the inner COLLABROOM payload
		// ({"type":"PAD_DELETE","data":{"padId":...}}), not the envelope.
		if padDelete, ok := decodeInto[PadDelete](inner, msgType, logger); ok {
			c.Handler.HandleMessage(padDelete, c, retrievedSettings, logger)
		}
	case msgSaveRevision:
		if saveRevision, ok := decodeInto[SavedRevision](message, msgType, logger); ok {
			c.Handler.HandleMessage(saveRevision, c, retrievedSettings, logger)
		}
	case msgUserChanges:
		if userChange, ok := decodeInto[ws.UserChange](message, msgType, logger); ok {
			c.Handler.HandleMessage(userChange, c, retrievedSettings, logger)
		}
	case msgSheetOp:
		if sheetOp, ok := decodeInto[ws.SheetOpIncoming](message, msgType, logger); ok {
			c.Handler.EnqueueSheetOp(c, sheetOp)
		}
	case msgSheetPresence:
		if presence, ok := decodeInto[ws.SheetPresenceIncoming](message, msgType, logger); ok {
			c.Handler.HandlePresence(c, presence)
		}
	case msgUserInfoUpdate:
		if userInfoChange, ok := decodeInto[UserInfoUpdateWrapper](message, msgType, logger); ok {
			c.Handler.HandleMessage(userInfoChange.Data, c, retrievedSettings, logger)
		}
	case msgGetChatMsgs:
		if getChatMessages, ok := decodeInto[ws.GetChatMessages](message, msgType, logger); ok {
			c.Handler.HandleMessage(getChatMessages, c, retrievedSettings, logger)
		}
	case msgChangesetReq:
		if changesetReq, ok := decodeInto[ws.ChangesetReq](message, msgType, logger); ok {
			c.Handler.HandleMessage(changesetReq, c, retrievedSettings, logger)
		}
	case msgChatMessage:
		if chatMessage, ok := decodeInto[ws.ChatMessage](message, msgType, logger); ok {
			c.Handler.HandleMessage(chatMessage, c, retrievedSettings, logger)
		}
	case msgClientMessage:
		if clientMessage, ok := decodeInto[ws.ClientMessage](message, msgType, logger); ok {
			c.Handler.HandleMessage(clientMessage, c, retrievedSettings, logger)
		}
	default:
		logger.Debugf("Ignoring unknown message type %q", msgType)
	}
}
