export type PadRecord = {
  padName: string
  revisionNumber: number
  lastEdited: number
  userCount: number
}

export type PluginRecord = {
  name: string
  description: string
  version: string
  enabled: boolean
}

export type UpdateCheckResult = {
  currentVersion: string
  latestVersion: string
  updateAvailable: boolean
}

export type ShoutMessage = {
  data: {
    payload: {
      timestamp: number
      message: {
        message: string
        sticky: boolean
      }
    }
  }
}

export type Toast = {
  kind: 'success' | 'error'
  message: string
} | null
