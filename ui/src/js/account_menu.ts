// The profile menu in the pad's toolbar: "Log in" for visitors, and the
// person's name with "Log out" once logged in (lib/api/padlogin). The
// server says what to show (/auth/me); a failure leaves the toolbar as it
// was and never stops the pad.

export interface Me {
  loginAvailable: boolean
  loggedIn: boolean
  username?: string
  displayName?: string
  admin?: boolean
  // tokens: may manage own API tokens ("My tokens").
  tokens?: boolean
}

export interface AccountMenuDeps {
  // The page's own URL: /auth sits next to /p/, also under a prefix.
  href: () => string
  fetch: (url: string) => Promise<{ok: boolean, json: () => Promise<unknown>}>
  t: (key: string, fallback: string) => string
  // The toolbar list the menu goes into, before the users button.
  toolbar: () => HTMLElement | null
  document: Document
  // openTokens shows "My tokens" (account_tokens.ts).
  openTokens?: () => void
}

export const authUrl = (href: string, page: 'login' | 'logout' | 'me'): string =>
  new URL(`../auth/${page}`, href).href

// back is where the login returns to: this page, path and query.
export const back = (href: string): string => {
  const u = new URL(href)
  return u.pathname + u.search
}

export const loginUrl = (href: string): string => {
  const u = new URL(authUrl(href, 'login'))
  u.searchParams.set('redirect', back(href))
  return u.href
}

export const initialOf = (me: Me): string => {
  const name = (me.displayName || me.username || '?').trim()
  return (Array.from(name)[0] || '?').toUpperCase()
}

const el = <K extends keyof HTMLElementTagNameMap>(
  doc: Document, tag: K, attrs: Record<string, string> = {}, text?: string): HTMLElementTagNameMap[K] => {
  const e = doc.createElement(tag)
  for (const [k, v] of Object.entries(attrs)) e.setAttribute(k, v)
  if (text !== undefined) e.textContent = text
  return e
}

// render builds the menu's list item, or null when there is nothing to
// show (logging in is not set up and nobody is logged in).
export const render = (me: Me, d: AccountMenuDeps): HTMLLIElement | null => {
  const doc = d.document
  const href = d.href()
  if (!me.loggedIn) {
    if (!me.loginAvailable) return null
    const li = el(doc, 'li', {'data-key': 'account', 'data-type': 'button'})
    const a = el(doc, 'a', {href: loginUrl(href), class: 'account-login', 'data-testid': 'account-login'},
      d.t('pad.account.login', 'Log in'))
    li.appendChild(a)
    return li
  }
  const name = me.displayName || me.username || ''
  const li = el(doc, 'li', {'data-key': 'account', 'data-type': 'button', class: 'account-item'})
  const button = el(doc, 'a', {
    role: 'button', tabindex: '0', class: 'account-button', 'data-testid': 'account-button',
    'aria-haspopup': 'menu', 'aria-expanded': 'false', title: name,
    'aria-label': `${d.t('pad.account.menu', 'Account')}: ${name}`,
  }, initialOf(me))
  const menu = el(doc, 'div', {class: 'account-menu', role: 'menu', 'data-testid': 'account-menu', hidden: ''})
  const who = el(doc, 'div', {class: 'account-who'})
  who.appendChild(el(doc, 'strong', {'data-testid': 'account-name'}, name))
  if (me.username && me.username !== name) who.appendChild(el(doc, 'span', {class: 'account-username'}, me.username))
  menu.appendChild(who)
  if (me.admin) {
    menu.appendChild(el(doc, 'a', {href: new URL('../admin/', href).href, role: 'menuitem', class: 'account-admin'},
      d.t('pad.account.admin', 'Admin')))
  }
  if (me.tokens && d.openTokens) {
    const openTokens = d.openTokens
    const item = el(doc, 'button', {type: 'button', role: 'menuitem', class: 'account-tokens-open', 'data-testid': 'account-tokens-open'},
      d.t('pad.account.tokens', 'My tokens'))
    item.addEventListener('click', () => { setOpen(false); openTokens() })
    menu.appendChild(item)
  }
  // A form, so logging out works without script errors and with the
  // site's own Origin, which the server checks.
  const form = el(doc, 'form', {method: 'post', action: authUrl(href, 'logout')})
  form.appendChild(el(doc, 'input', {type: 'hidden', name: 'redirect', value: back(href)}))
  form.appendChild(el(doc, 'button', {type: 'submit', role: 'menuitem', class: 'account-logout', 'data-testid': 'account-logout'},
    d.t('pad.account.logout', 'Log out')))
  menu.appendChild(form)

  let open = false
  // Fixed at the button: the toolbar clips what overflows it. Below the
  // button, or above it where the toolbar sits at the bottom (phones).
  const place = () => {
    const view = doc.defaultView
    if (!view) return
    const r = button.getBoundingClientRect()
    menu.style.right = `${Math.max(4, view.innerWidth - r.right)}px`
    if (r.top > view.innerHeight / 2) {
      menu.style.top = ''
      menu.style.bottom = `${view.innerHeight - r.top + 6}px`
    } else {
      menu.style.bottom = ''
      menu.style.top = `${r.bottom + 6}px`
    }
  }
  const setOpen = (o: boolean) => {
    open = o
    if (o) place()
    menu.hidden = !o
    button.setAttribute('aria-expanded', String(o))
  }
  doc.defaultView?.addEventListener('resize', () => { if (open) place() })
  button.addEventListener('click', (e) => { e.preventDefault(); setOpen(!open) })
  button.addEventListener('keydown', (e) => {
    if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setOpen(!open) }
    if (e.key === 'Escape') setOpen(false)
  })
  doc.addEventListener('click', (e) => { if (!li.contains(e.target as Node)) setOpen(false) })
  li.appendChild(button)
  li.appendChild(menu)
  return li
}

export const initAccountMenu = async (d: AccountMenuDeps): Promise<boolean> => {
  try {
    const res = await d.fetch(authUrl(d.href(), 'me'))
    if (!res.ok) return false
    const me = (await res.json()) as Me
    const li = render(me, d)
    const toolbar = d.toolbar()
    if (!li || !toolbar) return false
    toolbar.querySelector('li[data-key="account"]')?.remove()
    const users = toolbar.querySelector('li[data-key="showusers"]')
    toolbar.insertBefore(li, users)
    return true
  } catch {
    return false
  }
}
