/**
 * ep_font_color — Self-initializing EventBus subscriber.
 *
 * Provides font color formatting support via the editor EventBus.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

const colors = ['black', 'red', 'green', 'blue', 'yellow', 'orange'] as const
const colorRegex = /(?:^| )color:([A-Za-z0-9]*)/

// ---------------------------------------------------------------------------
// CSS injection — runs immediately at module load
// ---------------------------------------------------------------------------

const link = document.createElement('link')
link.rel = 'stylesheet'
link.href = '/static/plugins/ep_font_color/static/css/color.css'
document.head.appendChild(link)

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

const doInsertColors = function (this: any, level: number) {
  const rep = this.rep
  const documentAttributeManager = this.documentAttributeManager
  if (!(rep.selStart && rep.selEnd) || (level >= 0 && colors[level] === undefined)) return

  const newColor: [string, string] = level >= 0 ? ['color', colors[level]] : ['color', '']
  documentAttributeManager.setAttributesOnRange(rep.selStart, rep.selEnd, [newColor])
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
  context.editorInfo.ace_doInsertColors = doInsertColors.bind(context)
})

// Set up color dropdown UI when editor is ready
editorBus.on('editor:ready' as any, (context: { ace: any }) => {
  const btn = document.querySelector('[data-key="fontColor"]') as HTMLElement | null
  if (btn && !document.getElementById('font-color-dropdown')) {
    const picker = document.createElement('ep-color-picker')
    picker.id = 'font-color-dropdown'
    picker.setAttribute('colors', JSON.stringify(colors))
    picker.addEventListener('ep-color-select', ((e: CustomEvent) => {
      const idx = colors.indexOf(e.detail.color)
      if (idx >= 0) {
        context.ace.callWithAce((ace: any) => {
          ace.ace_doInsertColors(idx)
        }, 'insertColor', true)
      }
    }) as EventListener)

    btn.style.position = 'relative'
    btn.appendChild(picker)
  }
})

// Register fontColor command when toolbar is ready
editorBus.on('toolbar:ready' as any, (context: { toolbar: any }) => {
  context.toolbar.registerCommand('fontColor', () => {
    const dropdown = document.getElementById('font-color-dropdown')
    if (dropdown) dropdown.style.display = dropdown.style.display === 'none' ? '' : 'none'
  })

  // Register as a Web Component toolbar select via EventBus
  editorBus.emit('custom:toolbar:register:select' as any, {
    key: 'fontColor',
    title: 'Color',
    options: colors.map((c, i) => ({ label: c, value: String(i) })),
    onChange: (value: string) => {
      const ace = context.toolbar?.ace ?? (window as any).pad?.editor
      if (ace?.callWithAce) {
        ace.callWithAce((a: any) => {
          a.ace_doInsertColors(parseInt(value, 10))
        }, 'insertColor', true)
      }
    },
  })
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
