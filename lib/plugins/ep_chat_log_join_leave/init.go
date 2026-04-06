package ep_chat_log_join_leave

import (
	"sync"
	"time"

	"github.com/ether/etherpad-go/lib/hooks/events"
	"github.com/ether/etherpad-go/lib/plugins/interfaces"
	"github.com/ether/etherpad-go/lib/utils"
)

const disconnectTimeout = 10 * time.Second

type EpChatLogJoinLeavePlugin struct {
	enabled bool
	mu      sync.Mutex
	// users maps padId -> authorId -> pending leave timer (nil if connected)
	users map[string]map[string]*time.Timer
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
	p.users = make(map[string]map[string]*time.Timer)

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

func (p *EpChatLogJoinLeavePlugin) handleUserJoin(ctx *events.UserJoinLeaveContext) {
	p.mu.Lock()
	defer p.mu.Unlock()

	padUsers, ok := p.users[ctx.PadId]
	if ok {
		if timer, exists := padUsers[ctx.AuthorId]; exists && timer != nil {
			// User reconnected before the leave timeout fired — cancel it silently.
			timer.Stop()
			padUsers[ctx.AuthorId] = nil
			return
		}
	}

	// First time joining — send a join message.
	if padUsers == nil {
		padUsers = make(map[string]*time.Timer)
		p.users[ctx.PadId] = padUsers
	}
	padUsers[ctx.AuthorId] = nil

	now := time.Now().UnixMilli()
	ctx.BroadcastChat(map[string]any{
		"text":                   "",
		"userId":                 ctx.AuthorId,
		"time":                   now,
		"ep_chat_log_join_leave": "join",
	})
}

func (p *EpChatLogJoinLeavePlugin) handleUserLeave(ctx *events.UserJoinLeaveContext) {
	p.mu.Lock()

	padUsers, ok := p.users[ctx.PadId]
	if ok {
		if timer, exists := padUsers[ctx.AuthorId]; exists && timer != nil {
			timer.Stop()
		}
	} else {
		padUsers = make(map[string]*time.Timer)
		p.users[ctx.PadId] = padUsers
	}

	// Capture broadcast function for use in the timer goroutine.
	broadcastChat := ctx.BroadcastChat
	padId := ctx.PadId
	authorId := ctx.AuthorId

	padUsers[authorId] = time.AfterFunc(disconnectTimeout, func() {
		p.mu.Lock()
		defer p.mu.Unlock()

		if pu, ok := p.users[padId]; ok {
			delete(pu, authorId)
			if len(pu) == 0 {
				delete(p.users, padId)
			}
		}

		leaveTime := time.Now().Add(-disconnectTimeout).UnixMilli()
		broadcastChat(map[string]any{
			"text":                   "",
			"userId":                 authorId,
			"time":                   leaveTime,
			"ep_chat_log_join_leave": "leave",
		})
	})

	p.mu.Unlock()
}

var _ interfaces.EpPlugin = (*EpChatLogJoinLeavePlugin)(nil)
