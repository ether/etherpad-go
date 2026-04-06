import { useAdminSocket } from './useAdminSocket'

export function useAdminActions() {
  const { emit } = useAdminSocket()

  return {
    loadSettings: () => emit('load'),
    checkUpdates: () => emit('checkUpdates'),
    getInstalled: () => emit('getInstalled'),
    getStats: () => emit('getStats'),
    requestPads: (opts: {
      offset: number
      limit: number
      pattern: string
      sortBy: string
      ascending: boolean
    }) => emit('padLoad', opts),
    createPad: (padName: string) => emit('createPad', { padName }),
    deletePad: (padName: string) => emit('deletePad', padName),
    cleanupPadRevisions: (padName: string) => emit('cleanupPadRevisions', padName),
    sendBroadcast: (message: string, sticky: boolean) => emit('shout', { message, sticky }),
    saveSettings: (settings: string) => emit('saveSettings', settings),
    restartServer: () => emit('restartServer'),
    refreshAll: () => {
      emit('checkUpdates')
      emit('getInstalled')
      emit('getStats')
    },
  }
}
