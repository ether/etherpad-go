/**
 * ep_chat_log_join_leave — Self-initializing EventBus subscriber.
 *
 * Renders join/leave lines in the chat log and shows a short toast for them.
 * No hook exports — all behavior is registered via editorBus.on(...).
 *
 * The source is the server's join/leave chat message (lib/plugins/
 * ep_chat_log_join_leave): it is sent once when someone really joins or
 * leaves, and a quick reconnect (a phone waking up, a network switch) sends
 * nothing. The collab layer's user:join event is not used for toasts: it
 * also fires for every user who was already in the pad when this client
 * connects, and on every reconnect.
 */

import notifications from 'ep_etherpad-lite/static/js/notifications'
import html10n from 'ep_etherpad-lite/static/js/i18n'
import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'
import { siteUrl } from 'ep_etherpad-lite/static/js/core/basePath'
import { joinLeaveOf, pickName, type JoinLeave } from './joinLeave'

const defaultMsg: Record<JoinLeave, string> = {
  join: 'joined the pad',
  leave: 'left the pad',
}

const t = (type: JoinLeave): string => {
  const key = `ep_chat_log_join_leave-${type}`
  try {
    const text = html10n.get(key)
    if (text && text !== key) return text
  } catch {
    // translations not loaded yet
  }
  return defaultMsg[type]
}

// ---------------------------------------------------------------------------
// CSS injection — runs immediately at module load
// ---------------------------------------------------------------------------

const link = document.createElement('link')
link.rel = 'stylesheet'
link.href = siteUrl('/static/plugins/ep_chat_log_join_leave/static/css/index.css')
document.head.appendChild(link)

// ---------------------------------------------------------------------------
// Names: the server's message carries only the author id
// ---------------------------------------------------------------------------

const names = new Map<string, string>()
const remember = (data: { userId?: string; name?: string | null }) => {
  if (data?.userId && data.name) names.set(data.userId, data.name)
}
editorBus.on('user:join' as any, remember)
editorBus.on('user:info:updated' as any, remember)

const nameOf = (authorId: string, fallback: string): string => {
  const cv: any = (window as any).clientVars ?? {}
  return pickName(authorId, fallback, names, cv.collab_client_vars?.historicalAuthorData)
}

const ownUserId = (): string => String((window as any).clientVars?.userId ?? '')

// ---------------------------------------------------------------------------
// Chat log line + toast
// ---------------------------------------------------------------------------

export const renderJoinLeaveMessage = (context: {
  authorName: string
  author: string
  text: string
  message: any
  rendered: unknown
  sticky: boolean
  timestamp: number
  timeStr: string
  duration: number
  isHistoryAdd?: boolean
  unread?: boolean
}) => {
  const info = joinLeaveOf(context.message, context.author, !!context.isHistoryAdd, ownUserId())
  if (!info) return
  const type = info.type

  const typeId = `ep_chat_log_join_leave-${type}`
  // chat.ts already put the message's display name or "Unnamed" there.
  const authorName = nameOf(context.author, context.message?.displayName ?? context.authorName)
  context.authorName = authorName

  // A system line: no chat popup, not an unread message.
  context.duration = 0
  context.unread = false

  // Live and about someone else: a short toast. Never for oneself.
  if (info.toast) {
    notifications.add({
      text: `${authorName} ${t(type)}`,
      sticky: false,
      time: 4000,
      position: 'top',
    })
  }

  // Override the default chat log rendering.
  const timeElt = document.createElement('span')
  timeElt.classList.add('time')
  timeElt.append(context.timeStr)

  const nameElt = document.createElement('span')
  nameElt.classList.add('ep_chat_log_join_leave-name')
  nameElt.append(authorName)

  const msgElt = document.createElement('span')
  msgElt.classList.add('ep_chat_log_join_leave-message')
  msgElt.dataset.l10nId = typeId
  msgElt.append(t(type))

  context.rendered = document.createElement('p')
  const rendered = context.rendered as HTMLParagraphElement
  rendered.classList.add(typeId)
  rendered.append(timeElt, nameElt, ' ', msgElt)

  // Mimic default author class rendering.
  const authorClass = `author-${context.author.replace(/[^a-y0-9]/g, (c) => {
    if (c === '.') return '-'
    return `z${c.charCodeAt(0)}z`
  })}`
  rendered.classList.add(authorClass)
  rendered.dataset.authorId = context.author
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// chat:new:message carries the mutable chat context (chat.ts addMessage).
editorBus.on('chat:new:message' as any, (context: any) => {
  renderJoinLeaveMessage(context)
})
