import { useMemo } from 'react'
import { useEmit } from './useAdminSocket'

export function useAdminActions() {
  const emit = useEmit()

  return useMemo(() => ({
    loadSettings: () => emit('load'),
    checkUpdates: () => emit('checkUpdates'),
    getUpdateStatus: () => emit('getUpdateStatus'),
    applyUpdate: () => emit('applyUpdate'),
    acknowledgeUpdate: () => emit('acknowledgeUpdate'),
    cancelUpdate: () => emit('cancelUpdate'),
    getUpdateLog: () => emit('getUpdateLog'),
    sendTestMail: () => emit('sendTestMail'),
    loadAuthors: (q: { pattern: string; offset: number; limit: number; sortBy: string; ascending: boolean; includeErased: boolean }) =>
      emit('authorLoad', q),
    previewErasure: (authorID: string) => emit('anonymizeAuthorPreview', { authorID }),
    eraseAuthor: (authorID: string) => emit('anonymizeAuthor', { authorID }),
    getInstalled: () => emit('getInstalled'),
    getStats: () => emit('getStats'),
    requestPads: (opts: {
      offset: number
      limit: number
      pattern: string
      sortBy: string
      ascending: boolean
      filter?: string
    }) => emit('padLoad', opts),
    createPad: (padName: string) => emit('createPad', { padName }),
    deletePad: (padName: string) => emit('deletePad', padName),
    cleanupPadRevisions: (padName: string) => emit('cleanupPadRevisions', padName),
    sendBroadcast: (message: string, sticky: boolean) => emit('shout', { message, sticky }),
    loadApiKeys: () => emit('apiKeysLoad'),
    createApiKey: (description: string) => emit('apiKeyCreate', { description }),
    setApiKeyDisabled: (id: string, disabled: boolean) => emit('apiKeySetDisabled', { id, disabled }),
    deleteApiKey: (id: string) => emit('apiKeyDelete', { id }),
    loadAccounts: () => emit('accountsLoad'),
    createAccount: (a: { username: string; displayName?: string; email?: string; kind: 'human' | 'service'; role?: string; pads?: string }) =>
      emit('accountCreate', a),
    updateAccount: (a: { id: string; username?: string; displayName?: string; email?: string }) => emit('accountUpdate', a),
    resetPassword: (id: string) => emit('accountResetPassword', { id }),
    setAccountDisabled: (id: string, disabled: boolean) => emit('accountSetDisabled', { id, disabled }),
    deleteAccount: (id: string) => emit('accountDelete', { id }),
    addAssignment: (accountId: string, role: string, pads: string) => emit('assignmentAdd', { accountId, role, pads }),
    removeAssignment: (accountId: string, role: string, pads: string) => emit('assignmentRemove', { accountId, role, pads }),
    saveRole: (r: { id?: string; name: string; description: string; scopes: string[]; includes: string[]; limits: Record<string, number> }) =>
      emit('roleSave', r),
    deleteRole: (id: string) => emit('roleDelete', { id }),
    resetRole: (id: string) => emit('roleReset', { id }),
    applyTemplate: (template: 'open' | 'read' | 'closed') => emit('templateApply', { template }),
    loadAudit: (limit = 200) => emit('auditLoad', { limit }),
    saveSettings: (settings: string) => emit('saveSettings', settings),
    restartServer: () => emit('restartServer'),
    getConnections: () => emit('getConnections'),
    getSystemInfo: () => emit('getSystemInfo'),
    kickUser: (sessionId: string) => emit('kickUser', { sessionId }),
    searchPadContent: (query: string, limit?: number) => emit('searchPadContent', { query, limit: limit || 20 }),
    getPadContent: (padName: string) => emit('getPadContent', padName),
    bulkDeletePads: (padNames: string[]) => emit('bulkDeletePads', { padNames }),
    refreshAll: () => {
      emit('checkUpdates')
      emit('getUpdateStatus')
      emit('getInstalled')
      emit('getStats')
    },
  }), [emit])
}
