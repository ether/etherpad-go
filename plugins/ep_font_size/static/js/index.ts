/**
 * ep_font_size — Self-initializing EventBus subscriber.
 *
 * Provides font size formatting support via the editor EventBus.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

const sizes = ['8', '9', '10', '11', '12', '13', '14', '16', '18', '20', '24', '28', '36', '48', '60'] as const
const sizeRegex = /(?:^| )font-size:(\d+px)/

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
  const rep = this.rep
  const documentAttributeManager = this.documentAttributeManager
  if (!(rep.selStart && rep.selEnd) || (level >= 0 && sizes[level] === undefined)) return

  const newSize: [string, string] = level >= 0 ? ['font-size', sizes[level] + 'px'] : ['font-size', '']
  documentAttributeManager.setAttributesOnRange(rep.selStart, rep.selEnd, [newSize])
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
  context.editorInfo.ace_doInsertSizes = doInsertSizes.bind(context)
})

// Set up size dropdown UI when editor is ready
editorBus.on('editor:ready' as any, (context: { ace: any }) => {
  const btn = document.querySelector('[data-key="fontSize"]') as HTMLElement | null
  if (btn && !document.getElementById('font-size-dropdown')) {
    const dropdown = document.createElement('div')
    dropdown.id = 'font-size-dropdown'
    dropdown.style.display = 'none'
    dropdown.style.position = 'absolute'
    dropdown.style.zIndex = '1000'
    dropdown.style.background = '#fff'
    dropdown.style.border = '1px solid #ccc'
    dropdown.style.borderRadius = '4px'
    dropdown.style.padding = '4px'
    dropdown.style.boxShadow = '0 2px 8px rgba(0,0,0,0.15)'

    sizes.forEach((size, idx) => {
      const swatch = document.createElement('span')
      swatch.style.display = 'inline-block'
      swatch.style.padding = '2px 8px'
      swatch.style.margin = '2px'
      swatch.style.cursor = 'pointer'
      swatch.style.border = '1px solid #999'
      swatch.style.borderRadius = '3px'
      swatch.textContent = size
      swatch.title = size + 'px'
      swatch.addEventListener('click', () => {
        context.ace.callWithAce((ace: any) => {
          ace.ace_doInsertSizes(idx)
        }, 'insertSize', true)
        dropdown.style.display = 'none'
      })
      dropdown.appendChild(swatch)
    })

    btn.style.position = 'relative'
    btn.appendChild(dropdown)
  }
})

// Register fontSize command when toolbar is ready
editorBus.on('toolbar:ready' as any, (context: { toolbar: any }) => {
  context.toolbar.registerCommand('fontSize', () => {
    const dropdown = document.getElementById('font-size-dropdown')
    if (dropdown) dropdown.style.display = dropdown.style.display === 'none' ? '' : 'none'
  })

  // Register as a Web Component toolbar select via EventBus
  editorBus.emit('custom:toolbar:register:select' as any, {
    key: 'fontSize',
    title: 'Size',
    options: sizes.map((s, i) => ({ label: s + 'px', value: String(i) })),
    onChange: (value: string) => {
      const ace = context.toolbar?.ace ?? (window as any).pad?.editor
      if (ace?.callWithAce) {
        ace.callWithAce((a: any) => {
          a.ace_doInsertSizes(parseInt(value, 10))
        }, 'insertSize', true)
      }
    },
  })
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
