/**
 * ep_heading — Self-initializing EventBus subscriber.
 *
 * Provides heading (h1-h4, code) block formatting support via the editor EventBus.
 * Merges the former shared.ts content collection logic into this single file.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

const cssFiles = ['ep_heading/static/css/editor.css']
const tags = ['h1', 'h2', 'h3', 'h4', 'code'] as const
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
link.href = '/static/plugins/ep_heading/static/css/editor.css'
document.head.appendChild(link)

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

const range = (start: number, end: number): number[] =>
  Array.from({ length: Math.abs(end - start) + 1 }, (_, index) => start + index)

const applyHeading = (ace: any, value: string): void => {
  const level = Number.parseInt(value, 10)
  if (Number.isNaN(level)) return
  ace.ace_doInsertHeading(level)
}

const mountHeadingControl = () => {
  const item = document.querySelector<HTMLElement>('li[data-key="heading"]')
  const select = item?.querySelector<HTMLSelectElement>('select.heading-selection')
  if (!item || !select || item.querySelector('ep-toolbar-select')) return

  const options = Array.from(select.options).map((option) => ({
    label: option.textContent?.trim() || option.value,
    value: option.value,
  }))

  const control = document.createElement('ep-toolbar-select') as ToolbarSelectElement
  const label = select.getAttribute('aria-label') ?? item.getAttribute('title') ?? 'Heading'
  control.setAttribute('label', label)
  control.setAttribute('placeholder', label)
  control.options = options
  control.value = '-1'
  control.dataset.control = 'heading'
  control.addEventListener('ep-toolbar-select:change', ((event: CustomEvent) => {
    const value = String(event.detail?.value ?? '')
    if (!editorAce) return
    editorAce.callWithAce((ace: any) => {
      applyHeading(ace, value)
    }, 'insertheading', true)
    control.value = value
  }) as EventListener)

  item.replaceChildren(control)
  item.setAttribute('data-type', 'custom')
  item.removeAttribute('data-key')
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Inject CSS into the ACE editor iframe
editorBus.on('custom:ace:editor:css' as any, ({ result }: { result: string[] }) => {
  result.push(...cssFiles)
})

// Register block elements for headings (mutable result pattern)
editorBus.on('editor:register:block:elements' as any, ({ result }: { result: string[] }) => {
  result.push(...tags)
})

// Bind ace_doInsertHeading when the ACE editor is initialized
editorBus.on('editor:ace:initialized' as any, (context: any) => {
  context.editorInfo.ace_doInsertHeading = function (this: any, level: number): void {
    const rep = this.ace_getRep()
    if (!(rep.selStart && rep.selEnd)) return
    if (level >= 0 && tags[level] === undefined) return

    const firstLine = rep.selStart[0]
    const lastLine = Math.max(firstLine, rep.selEnd[0] - (rep.selEnd[1] === 0 ? 1 : 0))

    range(firstLine, lastLine).forEach((line) => {
      if (level >= 0) {
        this.ace_setAttributeOnLine(line, 'heading', tags[level])
      } else {
        this.ace_removeAttributeOnLine(line, 'heading')
      }
    })
  }
})

onDomReady(() => {
  mountHeadingControl()
})

// Set up heading toolbar UI when editor is ready
editorBus.on('editor:ready' as any, (context: { ace: any }) => {
  editorAce = context.ace
  mountHeadingControl()
})

// Track edit events to update the heading select UI
editorBus.on('editor:content:changed' as any, (call: any) => {
  if (!call?.callstack) return
  const cs = call.callstack
  if (cs.type !== 'handleClick' && cs.type !== 'handleKeyEvent' && !cs.docTextChanged) return
  if (cs.type === 'setBaseText' || cs.type === 'setup') return

  setTimeout(() => {
    const rep = call.rep
    if (!rep.selStart || !rep.selEnd) return

    const headingSelect = document.querySelector<ToolbarSelectElement>('ep-toolbar-select[data-control="heading"]')
    if (headingSelect) headingSelect.value = ''

    const attributeManager = call.documentAttributeManager
    const activeAttributes: Record<string, number> = {}
    const firstLine = rep.selStart[0]
    const lastLine = Math.max(firstLine, rep.selEnd[0] - (rep.selEnd[1] === 0 ? 1 : 0))
    let totalNumberOfLines = 0

    range(firstLine, lastLine).forEach((line) => {
      totalNumberOfLines += 1
      const attr = attributeManager.getAttributeOnLine(line, 'heading')
      if (!attr) return
      activeAttributes[attr] = (activeAttributes[attr] ?? 0) + 1
    })

    Object.entries(activeAttributes).forEach(([key, count]) => {
      if (count !== totalNumberOfLines || !headingSelect) return
      const index = tags.indexOf(key as (typeof tags)[number])
      if (index < 0) return
      headingSelect.value = String(index)
    })

    if (headingSelect && Object.keys(activeAttributes).length === 0) {
      headingSelect.value = '-1'
    }
  }, 250)
})

// Return heading classes for attribute-to-class mapping (mutable result pattern)
editorBus.on('editor:attribs:to:classes' as any, ({ key, value, result }: { key: string; value: string; result: string[] }) => {
  if (key === 'heading') {
    result.push(`heading:${value}`)
  }
})

// Process line attributes for heading DOM rendering (mutable result pattern)
editorBus.on('editor:process:line:attribs' as any, ({ cls, result }: { cls: string; result: any[] }) => {
  const headingType = /(?:^| )heading:([A-Za-z0-9]*)/.exec(cls)
  if (!headingType) return

  let tag = headingType[1]
  if (tag === 'h5' || tag === 'h6') tag = 'h4'
  if (!tags.includes(tag as (typeof tags)[number])) return

  result.push({
    preHtml: `<${tag}>`,
    postHtml: `</${tag}>`,
    processedMarker: true,
  })
})

// Collect content pre — handle heading tags during content collection
editorBus.on('editor:collect:content:pre' as any, (context: any) => {
  const { lineAttributes } = context.state
  const tagIndex = tags.indexOf(context.tname as (typeof tags)[number])

  if (context.tname === 'div' || context.tname === 'p') delete lineAttributes.heading
  if (tagIndex >= 0) lineAttributes.heading = tags[tagIndex]
})

// Collect content post — clean up heading line attributes after tag closes
editorBus.on('custom:collect:content:post' as any, (context: any) => {
  const { lineAttributes } = context.state
  const tagIndex = tags.indexOf(context.tname as (typeof tags)[number])
  if (tagIndex >= 0) delete lineAttributes.heading
})
