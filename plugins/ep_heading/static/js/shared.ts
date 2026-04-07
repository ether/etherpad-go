/**
 * ep_heading/shared — Content collection logic.
 *
 * This module's functionality has been merged into index.ts as EventBus
 * subscriptions. This file is kept as a no-op for backward compatibility
 * with any remaining ep.json references. All logic now lives in index.ts
 * and subscribes via editorBus.on('editor:collect:content:pre', ...) and
 * editorBus.on('custom:collect:content:post', ...).
 */
