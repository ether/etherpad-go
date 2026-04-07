/**
 * ep_font_size — Self-initializing EventBus subscriber.
 *
 * Provides font size formatting support via the editor EventBus.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

const sizes = ['8', '9', '10', '11', '12', '13', '14', '16', '18', '20', '24', '28', '36', '48', '60'] as const
const sizeRegex = /(?:^| )font-size:(\d+px)/
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
link.href = '/static/plugins/ep_font_size/static/css/size.css'
document.head.appendChild(link)

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

const doInsertSizes = function (this: any, level: number) {
  const rep = this.ace_getRep()
  if (!(rep.selStart && rep.selEnd) || (level >= 0 && sizes[level] === undefined)) return

  const newSize: [string, string] = level >= 0 ? ['font-size', sizes[level] + 'px'] : ['font-size', '']
  this.ace_performDocumentApplyAttributesToRange(rep.selStart, rep.selEnd, [newSize])
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Inject CSS into the ACE editor iframe
editorBus.on('custom:ace:editor:css' as any, ({ result }: { result: string[] }) => {
  result.push('ep_font_size/static/css/size.css')
})

// Bind ace_doInsertSizes when the ACE editor is initialized
editorBus.on('editor:ace:initialized' as any, (context: { editorInfo: any }) => {
  context.editorInfo.ace_doInsertSizes = doInsertSizes
})

const mountToolbarSelect = () => {
  const item = document.querySelector<HTMLElement>('li[data-key="fontSize"]')
  const select = item?.querySelector<HTMLSelectElement>('select.size-selection')
  if (!item || !select || item.querySelector('ep-toolbar-select')) return

  const control = document.createElement('ep-toolbar-select') as ToolbarSelectElement
  const label = select.getAttribute('aria-label') ?? item.getAttribute('title') ?? 'Font size'
  control.setAttribute('label', label)
  control.setAttribute('placeholder', label)
  control.setAttribute('icon-class', 'ep_font_size_icon')
  control.options = Array.from(select.options).map((option) => ({
    label: option.textContent?.trim() || option.value,
    value: option.value,
  }))
  control.addEventListener('ep-toolbar-select:change', ((event: CustomEvent) => {
    const level = Number.parseInt(String(event.detail?.value ?? ''), 10)
    if (Number.isNaN(level) || !editorAce) return
    editorAce.callWithAce((ace: any) => {
      ace.ace_doInsertSizes(level)
    }, 'insertSize', true)
    control.value = String(level)
  }) as EventListener)

  item.replaceChildren(control)
  item.setAttribute('data-type', 'custom')
  item.removeAttribute('data-key')
}

onDomReady(() => {
  mountToolbarSelect()
})

// Set up size dropdown UI when editor is ready
editorBus.on('editor:ready' as any, (context: { ace: any }) => {
  editorAce = context.ace
  mountToolbarSelect()
})

// Return size classes for attribute-to-class mapping (mutable result pattern)
editorBus.on('editor:attribs:to:classes' as any, ({ key, value, result }: { key: string; value: string; result: string[] }) => {
  if (key.includes('font-size:')) {
    const m = sizeRegex.exec(key)
    if (m) result.push(`font-size:${m[1]}`)
  }
  if (key === 'font-size') {
    result.push(`font-size:${value}`)
  }
})

// Create DOM line elements for size spans (mutable result pattern)
editorBus.on('editor:create:dom:line' as any, ({ cls, result }: { cls: string; result: any[] }) => {
  const m = sizeRegex.exec(cls)
  if (!m) return
  const sizeVal = m[1].replace('px', '')
  const idx = sizes.indexOf(sizeVal as typeof sizes[number])
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

// Collect content for size attributes (mutable result pattern)
editorBus.on('editor:collect:content:pre' as any, (context: any) => {
  const m = sizeRegex.exec(context.cls)
  if (m?.[1]) {
    context.cc.doAttrib(context.state, `font-size::${m[1]}`)
  }
})
