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
