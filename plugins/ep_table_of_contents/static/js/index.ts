/**
 * ep_table_of_contents — Self-initializing EventBus subscriber.
 *
 * Renders a Table of Contents sidebar from heading elements in the editor.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

// ---------------------------------------------------------------------------
// CSS injection — runs immediately at module load
// ---------------------------------------------------------------------------

const cssLink = document.createElement('link')
cssLink.rel = 'stylesheet'
cssLink.type = 'text/css'
cssLink.href = '/static/plugins/ep_table_of_contents/static/css/toc.css'
document.head.appendChild(cssLink)

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

let tocContainer: HTMLDivElement | null = null
let tocItemsContainer: HTMLDivElement | null = null

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

const getInnerDocument = (): Document | null => {
  const outerFrame = document.querySelector<HTMLIFrameElement>('iframe[name="ace_outer"]')
  const outerDoc = outerFrame?.contentDocument
  const innerFrame = outerDoc?.querySelector<HTMLIFrameElement>('iframe[name="ace_inner"]')
  return innerFrame?.contentDocument ?? null
}

const update = (): void => {
  if (!tocItemsContainer) return

  const innerDoc = getInnerDocument()
  if (!innerDoc) return

  const headings = innerDoc.querySelectorAll<HTMLElement>('h1, h2, h3, h4, h5, h6')
  const fragment = document.createDocumentFragment()

  // Determine which heading is currently active based on scroll position
  const innerBody = innerDoc.body
  const scrollTop = innerBody?.parentElement?.scrollTop ?? innerBody?.scrollTop ?? 0
  let activeHeading: HTMLElement | null = null

  headings.forEach((heading) => {
    if (heading.offsetTop <= scrollTop + 10) {
      activeHeading = heading
    }
  })

  headings.forEach((heading) => {
    const text = heading.textContent?.trim()
    if (!text) return

    const level = Number.parseInt(heading.tagName.charAt(1), 10)
    const item = document.createElement('div')
    item.className = `tocItem tocDepth${level}`
    item.textContent = text
    item.title = text

    if (heading === activeHeading) {
      item.classList.add('activeTOC')
    }

    item.addEventListener('click', () => {
      heading.scrollIntoView({ behavior: 'smooth', block: 'start' })
    })

    fragment.appendChild(item)
  })

  tocItemsContainer.innerHTML = ''
  tocItemsContainer.appendChild(fragment)
}

const enable = (): void => {
  if (tocContainer) {
    tocContainer.classList.add('active')
  }
}

const disable = (): void => {
  if (tocContainer) {
    tocContainer.classList.remove('active')
  }
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Initialize TOC UI when editor is ready
editorBus.on('editor:ready' as any, () => {
  // Create TOC container
  tocContainer = document.createElement('div')
  tocContainer.id = 'toc'

  tocItemsContainer = document.createElement('div')
  tocItemsContainer.id = 'tocItems'
  tocContainer.appendChild(tocItemsContainer)

  // Insert into DOM before #editorcontainer
  const editorContainer = document.getElementById('editorcontainer')
  if (editorContainer) {
    editorContainer.before(tocContainer)
  }

  // Bind settings checkbox
  const checkbox = document.querySelector<HTMLInputElement>('#options-toc')
  if (checkbox) {
    if (checkbox.checked) {
      enable()
    } else {
      disable()
    }

    checkbox.addEventListener('click', () => {
      if (checkbox.checked) {
        enable()
      } else {
        disable()
      }
    })
  }

  // Listen for scroll events in the inner editor to update active heading
  const innerDoc = getInnerDocument()
  if (innerDoc) {
    const scrollTarget = innerDoc.body?.parentElement ?? innerDoc.body
    if (scrollTarget) {
      scrollTarget.addEventListener('scroll', () => {
        update()
      })
    }
  }

  // Initial update
  setTimeout(() => {
    update()
  }, 200)
})

// Rebuild TOC when editor content changes
editorBus.on('editor:content:changed' as any, () => {
  setTimeout(() => {
    update()
  }, 100)
})
