package hooks

import (
	"github.com/gofiber/fiber/v2/utils"
)

type Hook struct {
	hooks map[string]map[string]func(hookName string, ctx any)
}

func NewHook() Hook {
	return Hook{
		hooks: make(map[string]map[string]func(hookName string, ctx any)),
	}
}

func (h *Hook) EnqueueHook(key string, ctx func(hookName string, ctx any)) string {
	var uuid = utils.UUID()
	var _, ok = h.hooks[key]

	if !ok {
		h.hooks[key] = make(map[string]func(hookName string, ctx any))
	}

	h.hooks[key][uuid] = ctx

	return uuid
}

func (h *Hook) DequeueHook(key, id string) {
	delete(h.hooks[key], id)
}

func (h *Hook) ExecuteHooks(key string, ctx any) {

	var _, ok = h.hooks[key]

	if !ok {
		return
	}

	for _, v := range h.hooks[key] {
		v(key, ctx)
	}
}

var HookInstance = NewHook()
