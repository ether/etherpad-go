/**
 * ep_clear_formatting — Self-initializing EventBus subscriber.
 *
 * Provides a "clear formatting" toolbar command that strips all non-author
 * attributes from the current selection.
 * No hook exports — all behavior is registered via editorBus.on(...).
 */

import { editorBus } from 'ep_etherpad-lite/static/js/core/EventBus'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

interface AceRep {
  selStart: [number, number]
  selEnd: [number, number]
  apool: {
    attribToNum: Record<string, number>
  }
}

interface ClearFormattingAce {
  ace_getRep: () => AceRep
  ace_setAttributeOnSelection: (attr: string, value: false) => void
  ace_doClearFormatting?: () => void
}

// ---------------------------------------------------------------------------
// EventBus subscriptions
// ---------------------------------------------------------------------------

// Bind ace_doClearFormatting when the ACE editor is initialized
editorBus.on('editor:ace:initialized' as any, (context: any) => {
  const doClearFormatting = function (this: any): void {
    const { rep } = this
    if (!rep.selStart || !rep.selEnd) return

    const isSelection =
      rep.selStart[0] !== rep.selEnd[0] || rep.selStart[1] !== rep.selEnd[1]
    if (!isSelection) return
  }

  context.editorInfo.ace_doClearFormatting = doClearFormatting.bind(context)
})

// Set up clear formatting button click handler when editor is ready
editorBus.on('editor:ready' as any, () => {
  // Click handler reserved for direct button clicks if needed
})

// Register clearFormatting command when toolbar is ready
editorBus.on('toolbar:ready' as any, (context: { toolbar: any; ace: any }) => {
  context.toolbar.registerCommand('clearFormatting', () => {
    context.ace.callWithAce((ace: any) => {
      const editor = ace as unknown as ClearFormattingAce
      const rep = editor.ace_getRep()

      const isSelection =
        rep.selStart[0] !== rep.selEnd[0] || rep.selStart[1] !== rep.selEnd[1]
      if (!isSelection) return

      const attrs = rep.apool.attribToNum
      for (const k of Object.keys(attrs)) {
        const attr = k.split(',')[0]
        if (attr !== 'author') {
          editor.ace_setAttributeOnSelection(attr, false)
        }
      }
    }, 'clearFormatting', true)
  })
})
