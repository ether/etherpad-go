package hooks


type PadDefaultContent struct {
	Type string
	Content *string
}


type Hook struct {
	PadDefaultContentHooks map[string] func(hookName string, ctx PadDefaultContent)
}


func (h *Hook) enqueueHook(key string, ctx any){
	if key == "padDefau"
}
