/**
 * ep_spellcheck — Self-initializing EventBus subscriber.
 *
 * Enables/disables browser spellcheck on the editor based on user preference.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

const COOKIE_NAME = 'prefs'

type Prefs = {
  spellcheck?: boolean
}

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

const setSpellcheck = (enabled: boolean): void => {
  const outerFrame = document.querySelector<HTMLIFrameElement>('iframe[name="ace_outer"]')
  const outerDocument = outerFrame?.contentDocument
  const innerFrame = outerDocument?.querySelector<HTMLIFrameElement>('iframe')
  const innerBody = innerFrame?.contentDocument?.querySelector<HTMLElement>('#innerdocbody')
  if (!innerBody) return

  innerBody.setAttribute('spellcheck', String(enabled))
  innerBody.querySelectorAll<HTMLElement>('div, span').forEach((node) => {
    node.setAttribute('spellcheck', String(enabled))
  })
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Initialize spellcheck when editor is ready
editorBus.on('editor:ready' as any, () => {
  const optionsSpellcheck = document.querySelector<HTMLInputElement>('#options-spellcheck')
  if (!optionsSpellcheck) return

  const prefs = parsePrefs()
  optionsSpellcheck.checked = prefs.spellcheck !== false
  setSpellcheck(optionsSpellcheck.checked)

  optionsSpellcheck.addEventListener('click', () => {
    const enabled = optionsSpellcheck.checked
    writePrefs({ ...prefs, spellcheck: enabled })
    setSpellcheck(enabled)
    window.location.reload()
  })
})

// Allow other plugins to toggle spellcheck via a custom bus event
editorBus.on('custom:spellcheck:toggle' as any, (data: { enabled: boolean }) => {
  const prefs = parsePrefs()
  writePrefs({ ...prefs, spellcheck: data.enabled })
  setSpellcheck(data.enabled)

  const optionsSpellcheck = document.querySelector<HTMLInputElement>('#options-spellcheck')
  if (optionsSpellcheck) {
    optionsSpellcheck.checked = data.enabled
  }
})
