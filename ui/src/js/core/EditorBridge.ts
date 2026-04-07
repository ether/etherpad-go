/**
 * EditorBridge — Bridges the existing pad/editor system with the EventBus.
 *
 * This module is the ONLY file that knows about both the legacy Etherpad modules
 * and the new EventBus.  It listens to existing objects (socket, collab_client,
 * editbar, chat) and re-emits events on the bus, and vice-versa.
 *
 * Imported once from pad.ts after the pad is fully initialised.
 * Does NOT modify any existing module — pure listener/bridge pattern.
 */

import { editorBus } from './EventBus';

// ---------------------------------------------------------------------------
// Types (keep loose — the legacy code is untyped)
// ---------------------------------------------------------------------------

interface UserInfo {
  userId: string;
  name?: string;
  colorId?: string;
}

interface CollabClient {
  setOnUserJoin: (cb: (info: UserInfo) => void) => void;
  setOnUserLeave: (cb: (info: UserInfo) => void) => void;
  setOnUpdateUserInfo: (cb: (info: UserInfo) => void) => void;
  setOnChannelStateChange: (cb: (state: string, message?: string) => void) => void;
  setOnClientMessage: (cb: (msg: any) => void) => void;
  setOnInternalAction: (cb: (action: string) => void) => void;
  sendMessage: (msg: any) => void;
}

interface Pad {
  collabClient: CollabClient;
  myUserInfo: UserInfo;
  handleUserJoin: (info: UserInfo) => void;
  handleUserLeave: (info: UserInfo) => void;
  handleUserUpdate: (info: UserInfo) => void;
  handleClientMessage: (msg: any) => void;
  handleChannelStateChange: (state: string, message?: string) => void;
  handleCollabAction: (action: string) => void;
}

interface Socket {
  on: (event: string, handler: (...args: any[]) => void) => void;
}

// ---------------------------------------------------------------------------
// State
// ---------------------------------------------------------------------------

/** Keeps track of unsubscribe functions so the bridge can be torn down. */
const teardownFns: Array<() => void> = [];

/** Stored pad reference — used by PluginAPI.callWithEditor() and similar. */
let padRef: Pad | null = null;

/** Returns the stored pad reference (may be null before init). */
export function getPadRef(): Pad | null {
  return padRef;
}

// ---------------------------------------------------------------------------
// Initialise
// ---------------------------------------------------------------------------

/**
 * Call once after `pad.collabClient` has been wired up and the socket is live.
 *
 * @param pad    The main `pad` object from pad.ts
 * @param socket The socket.io socket instance
 */
export function initEditorBridge(pad: Pad, socket: Socket): void {
  padRef = pad;

  // ------------------------------------------------------------------
  // 1. Socket events -> EventBus
  // ------------------------------------------------------------------

  const onConnect = () => {
    editorBus.emit('connection:connected');
  };

  const onDisconnect = (reason: any) => {
    editorBus.emit('connection:disconnected', {
      reason: typeof reason === 'string' ? reason : (reason?.reason ?? 'socket'),
    });
  };

  const onReconnectAttempt = () => {
    editorBus.emit('connection:reconnecting');
  };

  socket.on('connect', onConnect);
  socket.on('disconnect', onDisconnect);
  socket.on('reconnect_attempt', onReconnectAttempt);

  // ------------------------------------------------------------------
  // 2. Collab-client callbacks -> EventBus
  //
  //    The collab_client exposes `setOn*` setters.  The pad already
  //    registers its own handlers.  We wrap those handlers so that
  //    both the original logic AND the bus emission run.
  // ------------------------------------------------------------------

  const originalOnUserJoin = pad.handleUserJoin;
  pad.handleUserJoin = (info: UserInfo) => {
    originalOnUserJoin(info);
    editorBus.emit('user:join', {
      userId: info.userId,
      name: info.name,
      colorId: info.colorId,
    });
  };
  // Re-bind so collab_client sees the wrapped version.
  pad.collabClient.setOnUserJoin(pad.handleUserJoin);

  const originalOnUserLeave = pad.handleUserLeave;
  pad.handleUserLeave = (info: UserInfo) => {
    originalOnUserLeave(info);
    editorBus.emit('user:leave', { userId: info.userId });
  };
  pad.collabClient.setOnUserLeave(pad.handleUserLeave);

  const originalOnUserUpdate = pad.handleUserUpdate;
  pad.handleUserUpdate = (info: UserInfo) => {
    originalOnUserUpdate(info);
    editorBus.emit('user:info:updated', {
      userId: info.userId,
      name: info.name,
      colorId: info.colorId,
    });
  };
  pad.collabClient.setOnUpdateUserInfo(pad.handleUserUpdate);

  const originalOnChannelStateChange = pad.handleChannelStateChange;
  pad.handleChannelStateChange = (state: string, message?: string) => {
    originalOnChannelStateChange(state, message);
    if (state === 'CONNECTED') {
      editorBus.emit('connection:connected');
    } else if (state === 'RECONNECTING') {
      editorBus.emit('connection:reconnecting');
    } else if (state === 'DISCONNECTED') {
      editorBus.emit('connection:disconnected', { reason: message ?? 'unknown' });
    }
  };
  pad.collabClient.setOnChannelStateChange(pad.handleChannelStateChange);

  // ------------------------------------------------------------------
  // 3. EventBus -> existing editbar commands
  // ------------------------------------------------------------------

  const unsubToolbar = editorBus.on('toolbar:command', ({ command, value }) => {
    // Access the singleton editbar via the global — avoids importing
    // pad_editbar (which would create a coupling this file is meant to avoid).
    const editbar = (window as any).padeditbar;
    if (editbar?.triggerCommand) {
      editbar.triggerCommand(command, value);
    }
  });
  teardownFns.push(unsubToolbar);

  // ------------------------------------------------------------------
  // 4. EventBus -> chat controller
  // ------------------------------------------------------------------

  const unsubChat = editorBus.on('chat:message:sent', ({ text }) => {
    // Forward to existing chat send path via collabClient.
    if (pad.collabClient && text) {
      pad.collabClient.sendMessage({
        type: 'CHAT_MESSAGE',
        message: { text },
      });
    }
  });
  teardownFns.push(unsubChat);

  const unsubChatVisibility = editorBus.on('chat:visibility:changed', ({ visible }) => {
    const chatObj = (window as any).chat ?? (window as any).require?.('./chat')?.chat;
    if (!chatObj) return;
    if (visible) {
      chatObj.show?.();
    } else {
      chatObj.hide?.();
    }
  });
  teardownFns.push(unsubChatVisibility);

  // ------------------------------------------------------------------
  // 5. Settings changes -> EventBus
  // ------------------------------------------------------------------

  // The existing pad.changeViewOption / pad.changePadOption don't emit events,
  // but plugins or future UI can emit settings:changed on the bus and we
  // translate that to pad options.
  const unsubSettings = editorBus.on('settings:changed', ({ key, value }) => {
    const padAny = pad as any;
    if (padAny.changeViewOption) {
      padAny.changeViewOption(key, value);
    }
  });
  teardownFns.push(unsubSettings);
}

// ---------------------------------------------------------------------------
// Teardown (useful for tests / hot-reload)
// ---------------------------------------------------------------------------

export function destroyEditorBridge(): void {
  for (const fn of teardownFns) {
    try { fn(); } catch { /* ignore */ }
  }
  teardownFns.length = 0;
  padRef = null;
}
