import defs from './plugin_defs';

/**
 * Returns CSS class names for installed client-side plugins.
 * E.g. ['plugin-ep_align', 'plugin-ep_font_color']
 */
export const clientPluginNames = (): string[] => {
  const names = defs.parts
    .filter((part: any) => part.client_hooks)
    .map((part: any) => `plugin-${part.plugin}`);
  return [...new Set(names)];
};
