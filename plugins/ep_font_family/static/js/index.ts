/**
 * ep_font_family — Self-initializing EventBus subscriber.
 *
 * Provides font family formatting support via the editor EventBus.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

const fonts = ['arial', 'avant-garde', 'bookman', 'calibri', 'courier', 'garamond', 'helvetica', 'monospace', 'palatino', 'times-new-roman'] as const
const fontRegex = /(?:^| )font([a-z-]+)/
type ToolbarSelectElement = HTMLElement & {
  options: Array<{label: string; value: string}>;
  value: string;
}
let editorAce: any = null
const onDomReady = (fn: () => void) => {
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', fn, {once: true})
  } else {
    fn()
  }
}

// ---------------------------------------------------------------------------
// CSS injection — runs immediately at module load
// ---------------------------------------------------------------------------

const link = document.createElement('link')
link.rel = 'stylesheet'
link.href = '/static/plugins/ep_font_family/static/css/fonts.css'
document.head.appendChild(link)

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

const doInsertFonts = function (this: any, level: number) {
  const rep = this.ace_getRep()
  if (!(rep.selStart && rep.selEnd) || (level >= 0 && fonts[level] === undefined)) return

  const newFont: [string, string] = level >= 0 ? ['font-family', fonts[level]] : ['font-family', '']
  this.ace_performDocumentApplyAttributesToRange(rep.selStart, rep.selEnd, [newFont])
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Inject CSS into the ACE editor iframe
editorBus.on('custom:ace:editor:css' as any, ({ result }: { result: string[] }) => {
  result.push('ep_font_family/static/css/fonts.css')
})

// Bind ace_doInsertFonts when the ACE editor is initialized
editorBus.on('editor:ace:initialized' as any, (context: { editorInfo: any }) => {
  context.editorInfo.ace_doInsertFonts = doInsertFonts
})

const mountToolbarSelect = () => {
  const item = document.querySelector<HTMLElement>('li[data-key="fontFamily"]')
  const select = item?.querySelector<HTMLSelectElement>('select.font-family-selection')
  if (!item || !select || item.querySelector('ep-toolbar-select')) return

  const control = document.createElement('ep-toolbar-select') as ToolbarSelectElement
  const label = select.getAttribute('aria-label') ?? item.getAttribute('title') ?? 'Font family'
  control.setAttribute('label', label)
  control.setAttribute('placeholder', label)
  control.setAttribute('icon-class', 'ep_font_family_icon')
  control.options = Array.from(select.options).map((option) => ({
    label: option.textContent?.trim() || option.value,
    value: option.value,
  }))
  control.addEventListener('ep-toolbar-select:change', ((event: CustomEvent) => {
    const level = Number.parseInt(String(event.detail?.value ?? ''), 10)
    if (Number.isNaN(level) || !editorAce) return
    editorAce.callWithAce((ace: any) => {
      ace.ace_doInsertFonts(level)
    }, 'insertFont', true)
    control.value = String(level)
  }) as EventListener)

  item.replaceChildren(control)
  item.setAttribute('data-type', 'custom')
  item.removeAttribute('data-key')
}

onDomReady(() => {
  mountToolbarSelect()
})

// Set up font family dropdown UI when editor is ready
editorBus.on('editor:ready' as any, (context: { ace: any }) => {
  editorAce = context.ace
  mountToolbarSelect()
})

// Return font classes for attribute-to-class mapping (mutable result pattern)
editorBus.on('editor:attribs:to:classes' as any, ({ key, value, result }: { key: string; value: string; result: string[] }) => {
  if (key === 'font-family') {
    result.push('font' + value)
  }
})

// Create DOM line elements for font spans (mutable result pattern)
editorBus.on('editor:create:dom:line' as any, ({ cls, result }: { cls: string; result: any[] }) => {
  const m = fontRegex.exec(cls)
  if (!m) return
  const idx = fonts.indexOf(m[1] as typeof fonts[number])
  if (idx < 0) return
  result.push({ extraOpenTags: '', extraCloseTags: '', cls })
})

// Track edit events (content changes)
editorBus.on('editor:content:changed' as any, (call: any) => {
  if (!call?.callstack) return
  const cs = call.callstack
  if (!['handleClick', 'handleKeyEvent'].includes(cs.type) && !cs.docTextChanged) return
  if (cs.type === 'setBaseText' || cs.type === 'setup') return
})

// Collect content for font-family attributes (mutable result pattern)
editorBus.on('editor:collect:content:pre' as any, (context: any) => {
  const m = fontRegex.exec(context.cls)
  if (m?.[1]) {
    context.cc.doAttrib(context.state, `font-family::${m[1]}`)
  }
})
