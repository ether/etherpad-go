// @ts-nocheck
'use strict';

/**
 * Handler for the shoutMessage client message.
 * This is part of the ep_message_all functionality.
 */

const chat = require('./chat');

/**
 * Handles the shoutMessage event - displays a message to all users in the pad.
 * @param hookName - The name of the hook being called
 * @param context - The context object containing the message payload
 */
exports.handleClientMessage_shoutMessage = (hookName: string, context: any) => {
  const { payload } = context;

  if (!payload) {
    console.warn('shoutMessage received without payload');
    return;
  }

  // Display the shout message - could be shown in chat or as a notification
  if (payload.message) {
    // Option 1: Show in chat
    if (chat.chat && typeof chat.chat.addMessage === 'function') {
      chat.chat.addMessage({
        text: payload.message,
        userId: payload.userId || 'system',
        time: Date.now(),
        isShoutMessage: true,
      }, true, false);
    }

    // Option 2: Show as alert/notification (fallback)
    console.log(`[Shout Message]: ${payload.message}`);
  }
};


