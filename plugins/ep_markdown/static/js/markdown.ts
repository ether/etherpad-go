/**
 * ep_markdown — Self-initializing EventBus subscriber.
 *
 * Provides markdown mode toggle and export link setup.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

// ---------------------------------------------------------------------------
// CSS injection — runs immediately at module load
// ---------------------------------------------------------------------------

const cssLink = document.createElement('link')
cssLink.rel = 'stylesheet'
cssLink.href = '/static/plugins/ep_markdown/static/css/markdown.css'
document.head.appendChild(cssLink)

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

const getPadRootPath = (): string => {
  const match = /.*\/p\/[^/]+/.exec(document.location.pathname)
  if (match?.[0]) return match[0]
  return (window as any).clientVars?.padId ?? ''
}

const getInnerDocBody = (): HTMLElement | null => {
  const outerFrame = document.querySelector<HTMLIFrameElement>('iframe[name="ace_outer"]')
  const innerFrame = outerFrame?.contentDocument?.querySelector<HTMLIFrameElement>('iframe')
  return innerFrame?.contentDocument?.querySelector<HTMLElement>('#innerdocbody') ?? null
}

const setMarkdownMode = (enabled: boolean): void => {
  const body = getInnerDocBody()
  if (!body) return

  body.classList.toggle('markdown', enabled)

  const underlineButton = document.querySelector<HTMLElement>('#underline')
  const strikeButton = document.querySelector<HTMLElement>('#strikethrough')
  if (underlineButton) underlineButton.style.display = enabled ? 'none' : ''
  if (strikeButton) strikeButton.style.display = enabled ? 'none' : ''
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Inject CSS into the ACE editor iframe
editorBus.on('custom:ace:editor:css' as any, ({ result }: { result: string[] }) => {
  result.push('/ep_markdown/static/css/markdown.css')
})

// Initialize markdown mode when editor is ready
editorBus.on('editor:ready' as any, () => {
  const exportMarkdown = document.querySelector<HTMLAnchorElement>('#exportmarkdowna')
  if (exportMarkdown) exportMarkdown.href = `${getPadRootPath()}/export/markdown`

  const markdownCheckbox = document.querySelector<HTMLInputElement>('#options-markdown')
  if (!markdownCheckbox) return

  setMarkdownMode(markdownCheckbox.checked)
  markdownCheckbox.addEventListener('click', () => {
    setMarkdownMode(markdownCheckbox.checked)
  })
})
