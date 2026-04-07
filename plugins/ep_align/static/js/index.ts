/**
 * ep_align — Self-initializing EventBus subscriber.
 *
 * Provides text alignment (left, center, justify, right) formatting support.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

const tags = ['left', 'center', 'justify', 'right'] as const

// ---------------------------------------------------------------------------
// CSS injection — runs immediately at module load
// ---------------------------------------------------------------------------

const link = document.createElement('link')
link.rel = 'stylesheet'
link.href = '/static/plugins/ep_align/static/css/align.css'
document.head.appendChild(link)

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

const range = (start: number, end: number): number[] => {
  const length = Math.abs(end - start) + 1
  return Array.from({ length }, (_, index) => start + index)
}

const getAlignValue = (target: EventTarget | null): number | null => {
  if (!(target instanceof Element)) return null
  const button = target.closest<HTMLElement>('.ep_align')
  if (!button) return null
  const value = Number.parseInt(button.dataset.align ?? '', 10)
  return Number.isNaN(value) ? null : value
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Inject CSS into the ACE editor iframe
editorBus.on('custom:ace:editor:css' as any, ({ result }: { result: string[] }) => {
  result.push('ep_align/static/css/align.css')
})

// Register block elements for alignment (mutable result pattern)
editorBus.on('editor:register:block:elements' as any, ({ result }: { result: string[] }) => {
  result.push(...tags)
})

// Bind ace_doInsertAlign when the ACE editor is initialized
editorBus.on('editor:ace:initialized' as any, (context: any) => {
  const doInsertAlign = function (this: any, level: number): void {
    const { rep, documentAttributeManager } = this
    if (!(rep.selStart && rep.selEnd)) return
    if (level >= 0 && tags[level] === undefined) return

    const firstLine = rep.selStart[0]
    const lastLine = Math.max(firstLine, rep.selEnd[0] - (rep.selEnd[1] === 0 ? 1 : 0))
    range(firstLine, lastLine).forEach((line) => {
      if (level >= 0) {
        documentAttributeManager.setAttributeOnLine(line, 'align', tags[level])
      } else {
        documentAttributeManager.removeAttributeOnLine(line, 'align')
      }
    })
  }

  context.editorInfo.ace_doInsertAlign = doInsertAlign.bind(context)
})

// Set up alignment button click handlers when editor is ready
editorBus.on('editor:ready' as any, (context: { ace: any }) => {
  document.body.addEventListener('click', (event) => {
    const alignValue = getAlignValue(event.target)
    if (alignValue === null) return

    context.ace.callWithAce((ace: any) => {
      ace.ace_doInsertAlign(alignValue)
    }, 'insertalign', true)
  })
})

// Register alignment toolbar commands when toolbar is ready
editorBus.on('toolbar:ready' as any, (context: { toolbar: any; ace: any }) => {
  const align = (alignment: number): void => {
    context.ace.callWithAce((ace: any) => {
      ace.ace_doInsertAlign(alignment)
      ace.ace_focus()
    }, 'insertalign', true)
  }

  context.toolbar.registerCommand('alignLeft', () => align(0))
  context.toolbar.registerCommand('alignCenter', () => align(1))
  context.toolbar.registerCommand('alignJustify', () => align(2))
  context.toolbar.registerCommand('alignRight', () => align(3))
})

// Track edit events to update alignment UI state
editorBus.on('editor:content:changed' as any, (call: any) => {
  if (!call?.callstack) return
  const cs = call.callstack
  if (cs.type !== 'handleClick' && cs.type !== 'handleKeyEvent' && !cs.docTextChanged) return
  if (cs.type === 'setBaseText' || cs.type === 'setup') return

  setTimeout(() => {
    const rep = call.rep
    if (!rep.selStart || !rep.selEnd) return

    const attributeManager = call.documentAttributeManager
    const firstLine = rep.selStart[0]
    const lastLine = Math.max(firstLine, rep.selEnd[0] - (rep.selEnd[1] === 0 ? 1 : 0))
    const activeAttributes: Record<string, number> = {}
    let totalNumberOfLines = 0

    range(firstLine, lastLine + 1).forEach((line) => {
      totalNumberOfLines += 1
      const attr = attributeManager.getAttributeOnLine(line, 'align')
      if (!attr) return
      activeAttributes[attr] = (activeAttributes[attr] ?? 0) + 1
    })

    Object.entries(activeAttributes).forEach(([key, count]) => {
      if (count === totalNumberOfLines) {
        void key
      }
    })
  }, 250)
})

// Return alignment classes for attribute-to-class mapping (mutable result pattern)
editorBus.on('editor:attribs:to:classes' as any, ({ key, value, result }: { key: string; value: string; result: string[] }) => {
  if (key === 'align') {
    result.push(`align:${value}`)
  }
})

// Process line attributes for alignment DOM rendering (mutable result pattern)
editorBus.on('editor:process:line:attribs' as any, ({ cls, result }: { cls: string; result: any[] }) => {
  const alignType = /(?:^| )align:([A-Za-z0-9]*)/.exec(cls)
  if (!alignType) return

  const tag = alignType[1]
  if (!tags.includes(tag as (typeof tags)[number])) return

  const styles =
    'width:100%;margin:0 auto;list-style-position:inside;display:block;text-align:' + tag

  result.push({
    preHtml: `<${tag} style="${styles}">`,
    postHtml: `</${tag}>`,
    processedMarker: true,
  })
})
