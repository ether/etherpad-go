// "My tokens" in the pad's profile menu: the logged-in person's own API
// tokens (lib/api/padlogin/tokens.go). The server decides what a token may
// get; this dialog only offers what it was told and shows the answers.

export interface OwnToken {
  id: string
  name: string
  prefix: string
  scopes: string[]
  pads: string
  expiresAt: number | null
  createdAt: number
  lastUsedAt: number | null
  lastUsedIp?: string
  status: 'active' | 'grace'
}

export interface OwnScope {
  name: string
  description: string
  padScoped: boolean
}

export interface OwnTokens {
  tokens: OwnToken[]
  scopes: OwnScope[]
  requireExpiry: boolean
  maxLifetimeDays: number
  key?: string
  keyName?: string
  keyId?: string
  // reload: the change was made but the list could not be read.
  reload?: boolean
}

export interface Answer {
  ok: boolean
  status: number
  json: () => Promise<unknown>
}

export interface TokensDeps {
  href: () => string
  request: (url: string, init?: {method: string, body: string}) => Promise<Answer>
  t: (key: string, fallback: string) => string
  document: Document
  now?: () => number
  copy?: (text: string) => Promise<void>
}

export const tokensUrl = (href: string, ...rest: string[]): string =>
  new URL(['../auth/tokens', ...rest.map(encodeURIComponent)].join('/'), href).href

// expiryChoices are the lifetimes offered for a new token, in days (0 is
// "never"), within the policy, and the one chosen at first.
export const expiryChoices = (requireExpiry: boolean, maxDays: number): {days: number[], initial: number} => {
  let days = [7, 30, 90, 365]
  if (maxDays > 0) {
    days = days.filter((n) => n < maxDays)
    days.push(maxDays)
  } else if (!requireExpiry) {
    days.push(0)
  }
  return {days, initial: days.includes(30) ? 30 : days[days.length - 1]}
}

const DAY = 24 * 60 * 60 * 1000

// graceChoices are how long a rotated key keeps working, in days (0:
// stops at once), as on the admin page; never longer than the token had
// left.
export const graceChoices = (expiresAt: number | null, now: number): number[] =>
  [0, 1, 10, 20].filter((n) => expiresAt === null || now + n * DAY < expiresAt)

export const formatDate = (ms: number | null, never: string): string =>
  ms === null ? never : new Date(ms).toLocaleString(undefined, {dateStyle: 'medium', timeStyle: 'short'})

export const errorOf = async (a: Answer, fallback: string): Promise<string> => {
  try {
    const body = (await a.json()) as {error?: unknown}
    if (body && typeof body.error === 'string' && body.error) return body.error
  } catch { /* not JSON */ }
  return fallback
}

const el = <K extends keyof HTMLElementTagNameMap>(
  doc: Document, tag: K, attrs: Record<string, string> = {}, text?: string): HTMLElementTagNameMap[K] => {
  const e = doc.createElement(tag)
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v)
  if (text !== undefined) e.textContent = text
  return e
}

// openTokensDialog shows the dialog; it loads the list itself.
export const openTokensDialog = async (d: TokensDeps): Promise<HTMLDialogElement> => {
  const doc = d.document
  const t = d.t
  const now = d.now ?? Date.now
  doc.querySelector('dialog.account-tokens')?.remove()
  const dialog = el(doc, 'dialog', {class: 'account-tokens', 'data-testid': 'account-tokens',
    'aria-labelledby': 'account-tokens-title'})
  const head = el(doc, 'div', {class: 'account-tokens-head'})
  head.appendChild(el(doc, 'h2', {id: 'account-tokens-title'}, t('pad.tokens.title', 'My tokens')))
  const close = el(doc, 'button', {type: 'button', class: 'account-tokens-close', 'data-testid': 'tokens-close',
    'aria-label': t('pad.tokens.close', 'Close')}, '×')
  // Without <dialog> support (old browsers) close() is missing.
  const shut = () => (typeof dialog.close === 'function' ? dialog.close() : dialog.remove())
  close.addEventListener('click', shut)
  head.appendChild(close)
  dialog.appendChild(head)
  dialog.appendChild(el(doc, 'p', {class: 'account-tokens-intro'},
    t('pad.tokens.intro', 'Tokens let programs use the HTTP API in your name, with at most your rights.')))
  const error = el(doc, 'p', {class: 'account-tokens-error', role: 'alert', 'data-testid': 'tokens-error', hidden: ''})
  const shown = el(doc, 'div', {class: 'account-tokens-key', 'data-testid': 'tokens-key', hidden: ''})
  const list = el(doc, 'div', {class: 'account-tokens-list', 'data-testid': 'tokens-list'})
  const form = el(doc, 'form', {class: 'account-tokens-form', 'data-testid': 'tokens-form'})
  dialog.append(error, shown, list, form)
  dialog.addEventListener('close', () => dialog.remove())
  doc.body.appendChild(dialog)
  if (typeof dialog.showModal === 'function') dialog.showModal()
  else dialog.setAttribute('open', '')

  const fail = (msg: string) => { error.textContent = msg; error.hidden = false }
  // One request at a time: a second click on "Create" would make a second
  // token whose key replaces the first before it is copied.
  let busy = false
  const setBusy = (b: boolean) => {
    busy = b
    for (const c of dialog.querySelectorAll<HTMLButtonElement | HTMLSelectElement>('.account-tokens-list button, .account-tokens-list select, .account-tokens-form button')) {
      c.disabled = b
    }
  }
  const call = async (url: string, body?: unknown): Promise<void> => {
    if (busy) return
    error.hidden = true
    setBusy(true)
    try {
      let a: Answer
      try {
        a = await d.request(url, body === undefined ? undefined : {method: 'POST', body: JSON.stringify(body)})
      } catch {
        fail(t('pad.tokens.offline', 'The server could not be reached.'))
        return
      }
      if (!a.ok) {
        fail(await errorOf(a, t('pad.tokens.failed', 'That did not work.')))
        return
      }
      const st = (await a.json()) as OwnTokens
      if (st.reload) {
        // Keep what is on screen, show the key, then read the list again.
        if (st.key) showKey(st.key, st.keyName || '', st.keyId || '')
        setBusy(false)
        await call(tokensUrl(d.href()))
        return
      }
      show(st)
    } finally {
      setBusy(false)
    }
  }

  // shownId is the token whose key is on screen.
  let shownId = ''
  const showKey = (key: string, name: string, id: string) => {
    shownId = id
    shown.replaceChildren()
    shown.appendChild(el(doc, 'strong', {}, t('pad.tokens.newKey', 'Your new key for "{{name}}"').replace('{{name}}', name)))
    shown.appendChild(el(doc, 'p', {}, t('pad.tokens.keyOnce', 'Copy it now: it is not shown again.')))
    const row = el(doc, 'div', {class: 'account-tokens-keyrow'})
    const input = el(doc, 'input', {type: 'text', readonly: '', value: key, 'data-testid': 'tokens-key-value',
      'aria-label': t('pad.tokens.key', 'Key')})
    const copy = el(doc, 'button', {type: 'button', 'data-testid': 'tokens-copy'}, t('pad.tokens.copy', 'Copy'))
    copy.addEventListener('click', () => {
      input.select()
      const done = () => { copy.textContent = t('pad.tokens.copied', 'Copied') }
      const c = d.copy ?? ((s: string) => navigator.clipboard.writeText(s))
      c(key).then(done, () => doc.execCommand?.('copy') && done())
    })
    row.append(input, copy)
    shown.appendChild(row)
    shown.hidden = false
    // The form is below: the key would be out of sight.
    shown.scrollIntoView?.({block: 'nearest'})
    input.focus()
    input.select()
  }

  const show = (st: OwnTokens) => {
    if (st.key) showKey(st.key, st.keyName || '', st.keyId || '')
    else if (shownId && !st.tokens.some((tk) => tk.id === shownId)) {
      // Its token was revoked: the key is of no use any more.
      shownId = ''
      shown.replaceChildren()
      shown.hidden = true
    }
    renderList(st)
    renderForm(st)
  }

  const renderList = (st: OwnTokens) => {
    list.replaceChildren()
    if (st.tokens.length === 0) {
      list.appendChild(el(doc, 'p', {class: 'account-tokens-empty'}, t('pad.tokens.none', 'You have no tokens yet.')))
      return
    }
    const never = t('pad.tokens.never', 'never')
    for (const tk of st.tokens) {
      const item = el(doc, 'div', {class: 'account-token', 'data-testid': 'token', 'data-token-id': tk.id})
      const title = el(doc, 'div', {class: 'account-token-title'})
      title.appendChild(el(doc, 'strong', {}, tk.name))
      title.appendChild(el(doc, 'code', {}, `${tk.prefix}…`))
      if (tk.status === 'grace') {
        title.appendChild(el(doc, 'span', {class: 'account-token-grace', 'data-testid': 'token-grace'},
          t('pad.tokens.replaced', 'replaced')))
      }
      item.appendChild(title)
      const facts: [string, string][] = [
        [t('pad.tokens.scopes', 'Rights'), tk.scopes.join(', ')],
        [t('pad.tokens.pads', 'Pads'), tk.pads || t('pad.tokens.allPads', 'all pads')],
        [t('pad.tokens.expires', 'Expires'), formatDate(tk.expiresAt, never)],
        [t('pad.tokens.lastUsed', 'Last used'), formatDate(tk.lastUsedAt, never)],
      ]
      const dl = el(doc, 'dl')
      for (const [k, v] of facts) dl.append(el(doc, 'dt', {}, k), el(doc, 'dd', {}, v))
      item.appendChild(dl)
      const actions = el(doc, 'div', {class: 'account-token-actions'})
      if (tk.status === 'active') {
        const grace = el(doc, 'select', {'data-testid': 'token-grace-choice',
          'aria-label': t('pad.tokens.oldKey', 'Old key works')})
        for (const n of graceChoices(tk.expiresAt, now())) {
          const label = n === 0 ? t('pad.tokens.graceNone', 'Old key stops at once')
            : t('pad.tokens.graceDays', 'Old key works {{n}} more days').replace('{{n}}', String(n))
          grace.appendChild(el(doc, 'option', {value: String(n)}, label))
        }
        const rotate = el(doc, 'button', {type: 'button', 'data-testid': 'token-rotate'}, t('pad.tokens.rotate', 'New key'))
        rotate.addEventListener('click', () => {
          // Counted on the server's clock: the browser's may be off.
          void call(tokensUrl(d.href(), tk.id, 'rotate'), {oldDays: Number(grace.value)})
        })
        actions.append(grace, rotate)
      }
      const revoke = el(doc, 'button', {type: 'button', class: 'account-token-revoke', 'data-testid': 'token-revoke'},
        t('pad.tokens.revoke', 'Revoke'))
      let armed = false
      revoke.addEventListener('click', () => {
        if (!armed) {
          armed = true
          revoke.textContent = t('pad.tokens.revokeSure', 'Really revoke?')
          return
        }
        void call(tokensUrl(d.href(), tk.id, 'revoke'), {})
      })
      actions.appendChild(revoke)
      item.appendChild(actions)
      list.appendChild(item)
    }
  }

  const renderForm = (st: OwnTokens) => {
    form.replaceChildren()
    form.appendChild(el(doc, 'h3', {}, t('pad.tokens.issue', 'New token')))
    const name = el(doc, 'input', {type: 'text', required: '', maxlength: '100', 'data-testid': 'tokens-name'})
    const nameLabel = el(doc, 'label', {}, t('pad.tokens.name', 'Name'))
    nameLabel.appendChild(name)
    form.appendChild(nameLabel)
    const fieldset = el(doc, 'fieldset')
    fieldset.appendChild(el(doc, 'legend', {}, t('pad.tokens.scopes', 'Rights')))
    const boxes: HTMLInputElement[] = []
    for (const s of st.scopes) {
      const box = el(doc, 'input', {type: 'checkbox', value: s.name, 'data-testid': `tokens-scope-${s.name}`})
      if (s.name === 'pad:read') box.checked = true
      const label = el(doc, 'label', {title: s.description})
      label.append(box, doc.createTextNode(` ${s.name}`))
      label.appendChild(el(doc, 'span', {class: 'account-tokens-desc'}, t(`pad.tokens.scope.${s.name}`, s.description)))
      fieldset.appendChild(label)
      boxes.push(box)
    }
    form.appendChild(fieldset)
    const pads = el(doc, 'input', {type: 'text', placeholder: 'team/*', 'data-testid': 'tokens-pads'})
    const padsLabel = el(doc, 'label', {}, t('pad.tokens.padsOnly', 'Only these pads (empty: all you may use)'))
    padsLabel.appendChild(pads)
    form.appendChild(padsLabel)
    const expiry = el(doc, 'select', {'data-testid': 'tokens-expiry'})
    const choices = expiryChoices(st.requireExpiry, st.maxLifetimeDays)
    for (const days of choices.days) {
      const label = days === 0 ? t('pad.tokens.noExpiry', 'Never expires')
        : t('pad.tokens.days', '{{n}} days').replace('{{n}}', String(days))
      const o = el(doc, 'option', {value: String(days)}, label)
      if (days === choices.initial) o.selected = true
      expiry.appendChild(o)
    }
    const expiryLabel = el(doc, 'label', {}, t('pad.tokens.expires', 'Expires'))
    expiryLabel.appendChild(expiry)
    form.appendChild(expiryLabel)
    form.appendChild(el(doc, 'button', {type: 'submit', 'data-testid': 'tokens-issue'}, t('pad.tokens.create', 'Create token')))
    form.onsubmit = (e) => {
      e.preventDefault()
      const scopes = boxes.filter((b) => b.checked).map((b) => b.value)
      if (!name.value.trim()) { fail(t('pad.tokens.needName', 'Give the token a name.')); return }
      if (scopes.length === 0) { fail(t('pad.tokens.needScope', 'Choose at least one right.')); return }
      void call(tokensUrl(d.href()), {name: name.value.trim(), scopes, pads: pads.value.trim(),
        days: Number(expiry.value) || null})
    }
  }

  let first: Answer
  try {
    first = await d.request(tokensUrl(d.href()))
  } catch {
    fail(t('pad.tokens.offline', 'The server could not be reached.'))
    return dialog
  }
  if (!first.ok) fail(await errorOf(first, t('pad.tokens.failed', 'That did not work.')))
  else show((await first.json()) as OwnTokens)
  return dialog
}
