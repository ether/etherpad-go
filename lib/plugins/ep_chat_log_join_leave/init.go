package ep_chat_log_join_leave

import (
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

// disconnectTimeout is how long an author may be gone before "left" is
// said; a var for the tests.
var disconnectTimeout = 10 * time.Second

type EpChatLogJoinLeavePlugin struct {
	enabled bool
	mu      sync.Mutex
	// users maps padId -> authorId -> presence
	users map[string]map[string]*presence
}

func (p *EpChatLogJoinLeavePlugin) Name() string {
	return "ep_chat_log_join_leave"
}

func (p *EpChatLogJoinLeavePlugin) Description() string {
	return "Logs user join and leave events in the chat"
}

func (p *EpChatLogJoinLeavePlugin) SetEnabled(enabled bool) {
	p.enabled = enabled
}

func (p *EpChatLogJoinLeavePlugin) IsEnabled() bool {
	return p.enabled
}

func (p *EpChatLogJoinLeavePlugin) Init(store *interfaces.EpPluginStore) {
	store.Logger.Info("Initializing ep_chat_log_join_leave plugin")
	p.users = make(map[string]map[string]*presence)

	store.HookSystem.EnqueueHook("userJoin", func(ctx any) {
		event := ctx.(*events.UserJoinLeaveContext)
		p.handleUserJoin(event)
	})

	store.HookSystem.EnqueueHook("userLeave", func(ctx any) {
		event := ctx.(*events.UserJoinLeaveContext)
		p.handleUserLeave(event)
	})

	// Translation hook
	store.HookSystem.EnqueueGetPluginTranslationHooks(
		func(ctx *events.LocaleLoadContext) {
			loadedTranslations, err := utils.LoadPluginTranslations(
				ctx.RequestedLocale,
				store.UIAssets,
				"ep_chat_log_join_leave",
			)
			if err != nil {
				return
			}
			for k, v := range loadedTranslations {
				ctx.LoadedTranslations[k] = v
			}
		},
	)
}

// presence is one author in one pad: how many of their connections are
// open, and the pending "left" message after the last one closed.
type presence struct {
	conns int
	leave *time.Timer
}

// handleUserJoin announces an author when they really arrive: not for a
// second tab, and not when they come back before the leave timeout (a phone
// waking up, a network switch). A new connection that arrives before the old
// one is noticed as gone counts as the same visit too.
func (p *EpChatLogJoinLeavePlugin) handleUserJoin(ctx *events.UserJoinLeaveContext) {
	p.mu.Lock()
	defer p.mu.Unlock()

	padUsers := p.users[ctx.PadId]
	if padUsers == nil {
		padUsers = make(map[string]*presence)
		p.users[ctx.PadId] = padUsers
	}
	pr := padUsers[ctx.AuthorId]
	if pr == nil {
		pr = &presence{}
		padUsers[ctx.AuthorId] = pr
	}
	pr.conns++
	if pr.leave != nil {
		// Back before the leave timeout fired: cancel it silently.
		pr.leave.Stop()
		pr.leave = nil
		return
	}
	if pr.conns > 1 {
		// Already here (another tab, or the old connection not yet gone).
		return
	}

	ctx.BroadcastChat(map[string]any{
		"text":                   "",
		"userId":                 ctx.AuthorId,
		"time":                   time.Now().UnixMilli(),
		"ep_chat_log_join_leave": "join",
	})
}

// handleUserLeave says an author left once their last connection has been
// closed for disconnectTimeout.
func (p *EpChatLogJoinLeavePlugin) handleUserLeave(ctx *events.UserJoinLeaveContext) {
	p.mu.Lock()
	defer p.mu.Unlock()

	padUsers := p.users[ctx.PadId]
	pr := padUsers[ctx.AuthorId]
	if pr == nil || pr.conns == 0 {
		// A leave without a join we saw (e.g. the plugin started later).
		return
	}
	pr.conns--
	if pr.conns > 0 {
		return
	}

	broadcastChat := ctx.BroadcastChat
	padId := ctx.PadId
	authorId := ctx.AuthorId
	var timer *time.Timer
	timer = time.AfterFunc(disconnectTimeout, func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		pu := p.users[padId]
		cur := pu[authorId]
		if cur == nil || cur.leave != timer {
			// Came back in the meantime (Stop raced with the timer).
			return
		}
		delete(pu, authorId)
		if len(pu) == 0 {
			delete(p.users, padId)
		}

		broadcastChat(map[string]any{
			"text":                   "",
			"userId":                 authorId,
			"time":                   time.Now().Add(-disconnectTimeout).UnixMilli(),
			"ep_chat_log_join_leave": "leave",
		})
	})
	pr.leave = timer
}

var _ interfaces.EpPlugin = (*EpChatLogJoinLeavePlugin)(nil)
