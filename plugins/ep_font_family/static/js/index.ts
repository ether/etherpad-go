/**
 * ep_font_family — Self-initializing EventBus subscriber.
 *
 * Provides font family formatting support via the editor EventBus.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

const fonts = ['arial', 'avant-garde', 'bookman', 'calibri', 'courier', 'garamond', 'helvetica', 'monospace', 'palatino', 'times-new-roman'] as const
const fontLabels = ['Arial', 'Avant Garde', 'Bookman', 'Calibri', 'Courier', 'Garamond', 'Helvetica', 'Monospace', 'Palatino', 'Times New Roman'] as const
const fontRegex = /(?:^| )font([a-z-]+)/

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
  const rep = this.rep
  const documentAttributeManager = this.documentAttributeManager
  if (!(rep.selStart && rep.selEnd) || (level >= 0 && fonts[level] === undefined)) return

  const newFont: [string, string] = level >= 0 ? ['font-family', fonts[level]] : ['font-family', '']
  documentAttributeManager.setAttributesOnRange(rep.selStart, rep.selEnd, [newFont])
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
  context.editorInfo.ace_doInsertFonts = doInsertFonts.bind(context)
})

// Set up font family dropdown UI when editor is ready
editorBus.on('editor:ready' as any, (context: { ace: any }) => {
  const btn = document.querySelector('[data-key="fontFamily"]') as HTMLElement | null
  if (btn && !document.getElementById('font-family-dropdown')) {
    const dropdown = document.createElement('div')
    dropdown.id = 'font-family-dropdown'
    dropdown.style.display = 'none'
    dropdown.style.position = 'absolute'
    dropdown.style.zIndex = '1000'
    dropdown.style.background = '#fff'
    dropdown.style.border = '1px solid #ccc'
    dropdown.style.borderRadius = '4px'
    dropdown.style.padding = '4px'
    dropdown.style.boxShadow = '0 2px 8px rgba(0,0,0,0.15)'

    fonts.forEach((font, idx) => {
      const item = document.createElement('div')
      item.style.padding = '4px 8px'
      item.style.cursor = 'pointer'
      item.style.whiteSpace = 'nowrap'
      item.textContent = fontLabels[idx]
      item.title = fontLabels[idx]
      item.addEventListener('mouseenter', () => {
        item.style.backgroundColor = '#f0f0f0'
      })
      item.addEventListener('mouseleave', () => {
        item.style.backgroundColor = ''
      })
      item.addEventListener('click', () => {
        context.ace.callWithAce((ace: any) => {
          ace.ace_doInsertFonts(idx)
        }, 'insertFont', true)
        dropdown.style.display = 'none'
      })
      dropdown.appendChild(item)
    })

    btn.style.position = 'relative'
    btn.appendChild(dropdown)
  }
})

// Register fontFamily command when toolbar is ready
editorBus.on('toolbar:ready' as any, (context: { toolbar: any }) => {
  context.toolbar.registerCommand('fontFamily', () => {
    const dropdown = document.getElementById('font-family-dropdown')
    if (dropdown) dropdown.style.display = dropdown.style.display === 'none' ? '' : 'none'
  })

  // Register as a Web Component toolbar select via EventBus
  editorBus.emit('custom:toolbar:register:select' as any, {
    key: 'fontFamily',
    title: 'Font',
    options: fonts.map((f, i) => ({ label: fontLabels[i], value: String(i) })),
    onChange: (value: string) => {
      const ace = context.toolbar?.ace ?? (window as any).pad?.editor
      if (ace?.callWithAce) {
        ace.callWithAce((a: any) => {
          a.ace_doInsertFonts(parseInt(value, 10))
        }, 'insertFont', true)
      }
    },
  })
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
