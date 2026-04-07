/**
 * Core module re-exports.
 *
 * Usage:
 *   import { EventBus, editorBus, PluginAPI, BaseComponent } from '../core';
 */

export { EventBus, editorBus } from './EventBus';
export type { EditorEvents } from './EventBus';
export { PluginAPI } from './PluginAPI';
export type {
  ToolbarButtonConfig,
  ToolbarSelectConfig,
  SettingsItemConfig,
  NotificationConfig,
} from './PluginAPI';
export { BaseComponent } from './BaseComponent';
export { initEditorBridge, destroyEditorBridge, getPadRef } from './EditorBridge';
export { PluginBridgeAPI, getPluginAPI, disposeAllPluginAPIs, hookEventMap, eventHookMap } from './PluginBridge';
export { uiRegistry } from './registry';
export type {
  ToolbarButtonRegistration,
  ToolbarSelectRegistration,
  SettingsItemRegistration,
} from './registry';
