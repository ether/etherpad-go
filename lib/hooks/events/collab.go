package events

import "github.com/ether/etherpad-go/lib/models/clientVars"

// HandleMessageContext is passed to handleMessage hooks before an incoming
// socket message is dispatched. Message and Client are exposed as `any` to
// avoid the lib/ws -> lib/hooks import cycle; plugins type-assert them
// (Message to a concrete ws message type, Client to *ws.Client). A callback
// may call DropMessage() to stop the message from being processed.
type HandleMessageContext struct {
	Message  any
	Client   any
	PadId    string
	AuthorId string

	dropped bool
}

// DropMessage signals that the message must not be dispatched.
func (c *HandleMessageContext) DropMessage() { c.dropped = true }

// Dropped reports whether any callback dropped the message.
func (c *HandleMessageContext) Dropped() bool { return c.dropped }

// HandleMessageSecurityContext is passed to handleMessageSecurity hooks when a
// write message arrives on a read-only connection. A callback may call
// GrantWriteAccess() to allow this single message through. Message is `any`.
type HandleMessageSecurityContext struct {
	Message  any
	PadId    string
	AuthorId string

	writeGranted bool
}

// GrantWriteAccess allows this single write message despite the read-only connection.
func (c *HandleMessageSecurityContext) GrantWriteAccess() { c.writeGranted = true }

// WriteAccessGranted reports whether a callback granted write access.
func (c *HandleMessageSecurityContext) WriteAccessGranted() bool { return c.writeGranted }

// ClientReadyContext is passed to clientReady hooks once a client has finished
// joining a pad (informational).
type ClientReadyContext struct {
	PadId    string
	AuthorId string
	Token    string
}

// ClientVarsContext is passed to clientVars hooks just before the CLIENT_VARS
// payload is sent. A callback may mutate the typed ClientVars fields and/or add
// arbitrary top-level keys via Extra. On key collision the typed field wins
// (Extra cannot clobber engine-owned keys). The fire site is responsible for
// initializing Extra to a non-nil map before firing this hook.
type ClientVarsContext struct {
	ClientVars *clientVars.ClientVars
	Extra      map[string]any
	PadId      string
	AuthorId   string
}

// ChatNewMessageContext is passed to chatNewMessage hooks before a chat message
// is stored and broadcast. To change the text, set *ctx.Text = "..." (the
// canonical form); reassigning ctx.Text = &newString also works because the
// fire site reads ctx.Text back after the hooks run. A callback may call
// DropMessage() to suppress the message entirely. Message is the chat message
// exposed as `any`.
type ChatNewMessageContext struct {
	Message  any
	Text     *string
	PadId    string
	AuthorId string

	dropped bool
}

// DropMessage signals that the chat message must not be stored or broadcast.
func (c *ChatNewMessageContext) DropMessage() { c.dropped = true }

// Dropped reports whether any callback dropped the chat message.
func (c *ChatNewMessageContext) Dropped() bool { return c.dropped }
