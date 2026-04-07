/**
 * ep_font_color — Self-initializing EventBus subscriber.
 *
 * Provides font color formatting support via the editor EventBus.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

const colors = ['black', 'red', 'green', 'blue', 'yellow', 'orange'] as const
const colorRegex = /(?:^| )color:([A-Za-z0-9]*)/
type ToolbarSelectElement = HTMLElement & {
  options: Array<{label: string; value: string}>;
  value: string;
}
let editorAce: any = null
let lastExpandedSelection: { start: [number, number]; end: [number, number] } | null = null
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
link.href = '/static/plugins/ep_font_color/static/css/color.css'
document.head.appendChild(link)

const setToolbarColorIndicator = (control: HTMLElement, index: number) => {
  const selectedColor = colors[index] ?? colors[0]
  control.style.setProperty('--ep-font-color-swatch', selectedColor)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

const doInsertColors = function (this: any, level: number) {
  const rep = this.ace_getRep()
  if (!(rep.selStart && rep.selEnd) || (level >= 0 && colors[level] === undefined)) return

  const newColor: [string, string] = level >= 0 ? ['color', colors[level]] : ['color', '']
  const hasRangeSelection =
    rep.selStart[0] !== rep.selEnd[0] || rep.selStart[1] !== rep.selEnd[1]

  if (!hasRangeSelection && lastExpandedSelection) {
    this.ace_performDocumentApplyAttributesToRange(
      lastExpandedSelection.start,
      lastExpandedSelection.end,
      [newColor],
    )
    return
  }

  if (this.ace_setAttributeOnSelection) {
    this.ace_setAttributeOnSelection(newColor[0], newColor[1])
    return
  }
  this.ace_performDocumentApplyAttributesToRange(rep.selStart, rep.selEnd, [newColor])
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Inject CSS into the ACE editor iframe
editorBus.on('custom:ace:editor:css' as any, ({ result }: { result: string[] }) => {
  result.push('ep_font_color/static/css/color.css')
})

// Bind ace_doInsertColors when the ACE editor is initialized
editorBus.on('editor:ace:initialized' as any, (context: { editorInfo: any }) => {
  context.editorInfo.ace_doInsertColors = doInsertColors
})

const mountColorPicker = () => {
  const item = document.querySelector<HTMLElement>('li[data-key="fontColor"]')
  const select = item?.querySelector<HTMLSelectElement>('select.color-selection')
  if (!item || !select || item.querySelector('ep-toolbar-select')) return

  const control = document.createElement('ep-toolbar-select') as ToolbarSelectElement
  const label = select.getAttribute('aria-label') ?? item.getAttribute('title') ?? 'Font color'
  control.setAttribute('label', label)
  control.setAttribute('placeholder', label)
  control.setAttribute('icon-class', 'ep_font_color_icon')
  control.options = Array.from(select.options).map((option) => ({
    label: option.textContent?.trim() || option.value,
    value: option.value,
  }))
  setToolbarColorIndicator(control, 0)
  control.addEventListener('ep-toolbar-select:change', ((event: CustomEvent) => {
    const index = Number.parseInt(String(event.detail?.value ?? ''), 10)
    if (Number.isNaN(index) || !editorAce) return
    editorAce.callWithAce((ace: any) => {
      ace.ace_doInsertColors(index)
    }, 'insertColor', true)
    editorAce.focus?.()
    control.value = String(index)
    setToolbarColorIndicator(control, index)
  }) as EventListener)

  item.replaceChildren(control)
  item.setAttribute('data-type', 'custom')
  item.removeAttribute('data-key')
}

onDomReady(() => {
  mountColorPicker()
})

// Set up color dropdown UI when editor is ready
editorBus.on('editor:ready' as any, (context: { ace: any }) => {
  editorAce = context.ace
  mountColorPicker()
})

editorBus.on('editor:selection:changed' as any, ({ start, end }: { start: [number, number]; end: [number, number] }) => {
  const hasRangeSelection = start[0] !== end[0] || start[1] !== end[1]
  lastExpandedSelection = hasRangeSelection ? { start: [...start] as [number, number], end: [...end] as [number, number] } : null
})

// Return color classes for attribute-to-class mapping (mutable result pattern)
editorBus.on('editor:attribs:to:classes' as any, ({ key, value, result }: { key: string; value: string; result: string[] }) => {
  if (key.indexOf('color:') !== -1) {
    const m = colorRegex.exec(key)
    if (m) result.push(`color:${m[1]}`)
  }
  if (key === 'color') {
    result.push(`color:${value}`)
  }
})

// Create DOM line elements for color spans (mutable result pattern)
editorBus.on('editor:create:dom:line' as any, ({ cls, result }: { cls: string; result: any[] }) => {
  const m = colorRegex.exec(cls)
  if (!m) return
  const idx = colors.indexOf(m[1] as typeof colors[number])
  if (idx < 0) return
  result.push({ extraOpenTags: '', extraCloseTags: '', cls })
})

// Track edit events (content changes)
editorBus.on('editor:content:changed' as any, (call: any) => {
  if (!call?.callstack) return
  const cs = call.callstack
  if (['handleClick', 'handleKeyEvent'].indexOf(cs.type) === -1 && !cs.docTextChanged) return
  if (cs.type === 'setBaseText' || cs.type === 'setup') return
})

// Collect content for color attributes (mutable result pattern)
editorBus.on('editor:collect:content:pre' as any, (context: any) => {
  const m = colorRegex.exec(context.cls)
  if (m && m[1]) {
    context.cc.doAttrib(context.state, `color::${m[1]}`)
  }
})
