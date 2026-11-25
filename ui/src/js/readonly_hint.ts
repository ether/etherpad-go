// Read-only pads: the formatting buttons would do nothing, so the toolbar
// shows a hint in their place (CSS hides .menu_left on body.readonly).
//
// - Opened through a read-only link (r.…): "Read only".
// - Read-only because the visitor may not edit: "Read only", plus
//   "Log in to edit" when logging in is set up and nobody is logged in.
// A tap into the text also says so, briefly, instead of doing nothing.

import type {Me} from './account_menu';
import {loginUrl} from './account_menu';

export interface ReadOnlyHintDeps {
  document: Document
  href: () => string
  t: (key: string, fallback: string) => string
  // The toolbar the hint goes into, before the formatting buttons.
  toolbar: () => HTMLElement | null
  // Shows a short message (gritter). Optional: without it a tap stays silent.
  notify?: (text: string) => void
}

// isReadOnlyLink tells whether the page was opened through a read-only link.
export const isReadOnlyLink = (href: string): boolean => {
  const last = new URL(href).pathname.split('/').filter(Boolean).pop() ?? ''
  try {
    return decodeURIComponent(last).startsWith('r.')
  } catch {
    return last.startsWith('r.')
  }
}

const LOCK_SVG = '<svg viewBox="0 0 16 16" width="14" height="14" aria-hidden="true" focusable="false">' +
  '<path fill="currentColor" d="M8 1a3.5 3.5 0 0 0-3.5 3.5V6H4a1.5 1.5 0 0 0-1.5 1.5v6A1.5 1.5 0 0 0 4 15h8a1.5 1.5 0 0 0 1.5-1.5v-6A1.5 1.5 0 0 0 12 6h-.5V4.5A3.5 3.5 0 0 0 8 1Zm2 5H6V4.5a2 2 0 1 1 4 0V6Z"/></svg>'

export const renderReadOnlyHint = (d: ReadOnlyHintDeps): HTMLElement => {
  const doc = d.document
  const hint = doc.createElement('div')
  hint.className = 'readonly-hint'
  hint.setAttribute('role', 'status')
  hint.setAttribute('data-testid', 'readonly-hint')
  const icon = doc.createElement('span')
  icon.className = 'readonly-hint-icon'
  icon.innerHTML = LOCK_SVG
  const label = doc.createElement('span')
  label.className = 'readonly-hint-label'
  label.textContent = d.t('pad.readonly.hint', 'Read only')
  hint.append(icon, label)
  return hint
}

// offerLogin adds "Log in to edit" when that would help.
export const offerLogin = (hint: HTMLElement, me: Me, d: ReadOnlyHintDeps): boolean => {
  if (me.loggedIn || !me.loginAvailable || isReadOnlyLink(d.href())) return false
  if (hint.querySelector('.readonly-hint-login')) return true
  const doc = d.document
  const sep = doc.createElement('span')
  sep.className = 'readonly-hint-sep'
  sep.setAttribute('aria-hidden', 'true')
  sep.textContent = '·'
  const a = doc.createElement('a')
  a.className = 'readonly-hint-login'
  a.setAttribute('data-testid', 'readonly-hint-login')
  a.href = loginUrl(d.href())
  a.textContent = d.t('pad.readonly.login', 'Log in to edit')
  hint.append(sep, a)
  return true
}

export const showReadOnlyHint = (d: ReadOnlyHintDeps): HTMLElement | null => {
  const toolbar = d.toolbar()
  if (!toolbar) return null
  const existing = toolbar.querySelector<HTMLElement>('.readonly-hint')
  if (existing) return existing
  const hint = renderReadOnlyHint(d)
  toolbar.insertBefore(hint, toolbar.querySelector('ul.menu_left'))
  return hint
}

// tellOnTap shows a short notice when someone taps into the read-only text.
// At most once every few seconds, so a scroll gesture does not stack them.
export const tellOnTap = (target: HTMLElement | null, d: ReadOnlyHintDeps, now = () => Date.now()) => {
  if (!target || !d.notify) return
  const notify = d.notify
  let last = -Infinity
  target.addEventListener('click', (e) => {
    const el = e.target as Element | null
    if (el?.closest?.('a[href], #sidediv')) return
    const sel = d.document.getSelection()
    if (sel && !sel.isCollapsed) return
    if (now() - last < 4000) return
    last = now()
    notify(d.t('pad.readonly.tap', 'This pad is read-only.'))
  })
}
