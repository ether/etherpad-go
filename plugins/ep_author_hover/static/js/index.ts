/**
 * ep_author_hover — Self-initializing EventBus subscriber.
 *
 * Shows a tooltip with the author name when hovering over authored text
 * in the editor. Controlled via a settings checkbox and cookie preference.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

// ---------------------------------------------------------------------------
// Constants & state
// ---------------------------------------------------------------------------

const COOKIE_NAME = 'prefs'

type Prefs = {
  'author-hover'?: boolean
}

let enabled = true
let hoverTimer: ReturnType<typeof setTimeout> | null = null

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

const parsePrefs = (): Prefs => {
  const cookie = document.cookie
    .split(';')
    .map((part) => part.trim())
    .find((part) => part.startsWith(`${COOKIE_NAME}=`))

  if (!cookie) return {}

  try {
    const value = decodeURIComponent(cookie.slice(`${COOKIE_NAME}=`.length))
    return JSON.parse(value) as Prefs
  } catch {
    return {}
  }
}

const writePrefs = (prefs: Prefs): void => {
  const encoded = encodeURIComponent(JSON.stringify(prefs))
  document.cookie = `${COOKIE_NAME}=${encoded}; path=/; SameSite=Lax`
}

/**
 * Decode an author class name (e.g. "author-a.JKu4Re7z8B2P3bZB") into the
 * raw author id.
 */
const authorIdFromClass = (className: string): string | null => {
  const classes = className.split(/\s+/)
  for (const cls of classes) {
    if (cls.startsWith('author-')) {
      const encoded = cls.substring(7)
      const decoded = encoded.replace(/[a-y0-9]+|-|z.+?z/g, (cc: string) => {
        if (cc === '-') return '.'
        if (cc.charAt(0) === 'z') return String.fromCharCode(Number(cc.slice(1, -1)))
        return cc
      })
      return decoded
    }
  }
  return null
}

/**
 * Look up an author's display name and color.
 */
const getAuthorInfo = (authorId: string): { name: string; color: string } => {
  const clientVars = window.clientVars as Record<string, unknown>
  const shortId = authorId.substring(0, 14)

  // Check if it's the current user
  const myUserId = (clientVars.userId ?? '') as string
  if (myUserId.substring(0, 14) === shortId) {
    return { name: getTranslation('ep_author_hover.me', 'Me'), color: '#fff' }
  }

  // Look up in the connected-users table
  const rows = document.querySelectorAll<HTMLTableRowElement>('#otheruserstable > tbody > tr')
  for (const row of rows) {
    const rowAuthorId = row.dataset.authorid ?? ''
    if (rowAuthorId.substring(0, 14) === shortId) {
      const nameEl = row.querySelector('.usertdname')
      const colorEl = row.querySelector<HTMLElement>('.usertdswatch > div')
      const name = nameEl?.textContent?.trim() || getTranslation('ep_author_hover.unknow_author', 'Unknown Author')
      const color = colorEl?.style.backgroundColor ?? '#fff'
      return { name, color }
    }
  }

  // Fall back to historical author data
  const collabVars = clientVars.collab_client_vars as Record<string, unknown> | undefined
  const historicalData = collabVars?.historicalAuthorData as Record<string, { name?: string; colorId?: string }> | undefined
  if (historicalData && historicalData[authorId]) {
    const hist = historicalData[authorId]
    return {
      name: hist.name || getTranslation('ep_author_hover.unknow_author', 'Unknown Author'),
      color: hist.colorId ?? '#fff',
    }
  }

  return { name: getTranslation('ep_author_hover.unknow_author', 'Unknown Author'), color: '#fff' }
}

const getTranslation = (key: string, fallback: string): string => {
  const el = document.querySelector(`[data-l10n-id="${key}"]`)
  if (el?.textContent?.trim()) return el.textContent.trim()
  return fallback
}

const destroyTooltip = (): void => {
  const outerFrame = document.querySelector<HTMLIFrameElement>('iframe[name="ace_outer"]')
  const outerBody = outerFrame?.contentDocument?.body
  if (!outerBody) return
  outerBody.querySelectorAll('.authortooltip').forEach((el) => el.remove())
}

const drawTooltip = (event: MouseEvent, authorName: string, authorColor: string): void => {
  if (!authorName) return

  const outerFrame = document.querySelector<HTMLIFrameElement>('iframe[name="ace_outer"]')
  const outerBody = outerFrame?.contentDocument?.body
  if (!outerBody) return

  const innerFrame = outerFrame?.contentDocument?.querySelector<HTMLIFrameElement>('iframe[name="ace_inner"]')
  if (!innerFrame) return

  const span = event.target as HTMLElement
  let top = span.offsetTop - 14
  if (top < 0) top = span.offsetHeight + 14

  let left = event.clientX + 15

  const inFramePos = innerFrame.getBoundingClientRect()
  left += inFramePos.left
  top += inFramePos.top

  const tooltip = document.createElement('div')
  tooltip.className = 'authortooltip'
  tooltip.title = authorName
  tooltip.textContent = authorName
  tooltip.style.position = 'absolute'
  tooltip.style.left = `${left}px`
  tooltip.style.top = `${top}px`
  tooltip.style.backgroundColor = authorColor
  tooltip.style.opacity = '0.85'
  tooltip.style.fontSize = '13px'
  tooltip.style.padding = '4px 8px'
  tooltip.style.borderRadius = '4px'
  tooltip.style.color = '#333'
  tooltip.style.boxShadow = '0 1px 4px rgba(0,0,0,0.25)'
  tooltip.style.pointerEvents = 'none'
  tooltip.style.whiteSpace = 'nowrap'
  tooltip.style.zIndex = '10000'

  outerBody.appendChild(tooltip)

  // Fade out and remove after a short delay
  setTimeout(() => {
    tooltip.style.transition = 'opacity 0.5s ease'
    tooltip.style.opacity = '0'
    setTimeout(() => tooltip.remove(), 500)
  }, 700)
}

const showAuthor = (event: MouseEvent): void => {
  if (!enabled) return

  const target = (event.target as HTMLElement).closest('span')
  if (!target) return

  const authorId = authorIdFromClass(target.className)
  if (!authorId) return

  destroyTooltip()

  const { name, color } = getAuthorInfo(authorId)
  drawTooltip(event, name, color)

  // Emit a custom event for other plugins (e.g. analytics, extended author cards)
  editorBus.emit('custom:author-hover:shown' as any, {
    authorId,
    name,
    color,
  })
}

const onMouseMove = (event: MouseEvent): void => {
  if (hoverTimer) {
    clearTimeout(hoverTimer)
    hoverTimer = null
  }
  hoverTimer = setTimeout(() => showAuthor(event), 1000)
}

const attachListener = (): void => {
  const outerFrame = document.querySelector<HTMLIFrameElement>('iframe[name="ace_outer"]')
  const outerDoc = outerFrame?.contentDocument
  const innerFrame = outerDoc?.querySelector<HTMLIFrameElement>('iframe')
  const innerBody = innerFrame?.contentDocument?.querySelector<HTMLElement>('#innerdocbody')
  if (!innerBody) return

  innerBody.addEventListener('mousemove', onMouseMove)
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Initialize author hover when editor is ready
editorBus.on('editor:ready' as any, () => {
  const checkbox = document.querySelector<HTMLInputElement>('#options-author-hover')
  if (!checkbox) return

  // Restore preference from cookie
  const prefs = parsePrefs()
  if (prefs['author-hover'] === false) {
    checkbox.checked = false
    enabled = false
  } else {
    checkbox.checked = true
    enabled = true
  }

  // Toggle on click
  checkbox.addEventListener('click', () => {
    enabled = checkbox.checked
    writePrefs({ ...prefs, 'author-hover': enabled })
  })

  // Attach the hover listener to the editor
  attachListener()
})

// Destroy stale tooltips when author info changes
editorBus.on('user:info:updated' as any, () => {
  destroyTooltip()
})
