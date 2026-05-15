import { editorBus } from './EventBus'
import { EpNotification, EpToastContainer } from 'etherpad-webcomponents'

// Initialize toast container
const toasts = EpToastContainer.getInstance()

// Show notifications for plugin events
editorBus.on('custom:notification:show', (data: any) => {
  EpNotification.show({
    text: data.text,
    type: data.type ?? 'info',
    duration: data.duration ?? 3000,
    position: data.position ?? 'top',
  })
})

// Connection status notifications
editorBus.on('connection:connected', () => {
  toasts.addToast({ message: 'Connected', type: 'success', duration: 2000 })
})

editorBus.on('connection:disconnected', ({ reason }) => {
  if (reason === 'kicked') return // handled by modal
  toasts.addToast({ message: `Disconnected: ${reason}`, type: 'error', duration: 0 })
})

// Plugin loaded notifications (debug)
editorBus.on('plugin:loaded', ({ name }) => {
  if ((editorBus as any).debug) {
    toasts.addToast({ message: `Plugin loaded: ${name}`, type: 'info', duration: 1500 })
  }
})
