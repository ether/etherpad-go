// The decisions of ep_chat_log_join_leave, without DOM or globals, so they
// can be tested (ui/vitest.config.ts includes this plugin's tests).

export type JoinLeave = 'join' | 'leave'

export interface JoinLeaveInfo {
  type: JoinLeave
  // A toast only for a live message about someone else.
  toast: boolean
}

// joinLeaveOf tells whether a chat message is the server's join/leave
// message (lib/plugins/ep_chat_log_join_leave), and whether it deserves a
// toast. null for every other chat message.
export const joinLeaveOf = (
  message: unknown, author: string, isHistoryAdd: boolean, ownUserId: string,
): JoinLeaveInfo | null => {
  const type = (message as {ep_chat_log_join_leave?: unknown} | null)?.ep_chat_log_join_leave
  if (type !== 'join' && type !== 'leave') return null
  return {type, toast: !isHistoryAdd && author !== ownUserId}
}

// pickName: the name last seen for this author (user list), else the one in
// the pad's author data, else the fallback (the message's display name or
// "Unnamed"), else the id.
export const pickName = (
  authorId: string, fallback: string, names: ReadonlyMap<string, string>,
  historical?: Record<string, {name?: string | null} | undefined> | null,
): string => names.get(authorId) || historical?.[authorId]?.name || fallback || authorId
