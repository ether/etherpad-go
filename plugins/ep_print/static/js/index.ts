/**
 * ep_print — Self-initializing EventBus subscriber.
 *
 * Adds a print command to the toolbar and injects print CSS.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

// ---------------------------------------------------------------------------
// CSS injection — runs immediately at module load
// ---------------------------------------------------------------------------

const link = document.createElement('link')
link.rel = 'stylesheet'
link.type = 'text/css'
link.href = '../static/plugins/ep_print/static/css/print.css'
link.media = 'print'
document.head.appendChild(link)

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Register print command when toolbar is ready
editorBus.on('toolbar:ready' as any, (context: { toolbar: any }) => {
  context.toolbar.registerCommand('print', () => {
    window.print()
    editorBus.emit('custom:print:triggered' as any, {})
  })
})
