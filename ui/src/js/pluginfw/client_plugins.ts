import defs from './plugin_defs';

export let baseURL = '';
export const setBaseURL = (value: string): void => {
  baseURL = value;
};

export const ensure = (cb: () => void): void => {
  if (!defs.loaded) {
    void update().then(cb);
    return;
  }
  cb();
};

/**
 * Load plugin definitions (metadata for admin UI and plugin-definitions.json).
 * Plugins self-initialize when imported via dynamic imports in plugin_registry.ts.
 */
export const update = async (): Promise<void> => {
  const response = await fetch(
    `${baseURL}pluginfw/plugin-definitions.json?v=${window.clientVars.randomVersionString}`);
  if (!response.ok) {
    throw new Error(`Failed to load plugin definitions (${response.status})`);
  }
  const data = await response.json();
  defs.plugins = data.plugins;
  defs.parts = data.parts;
  defs.loaded = true;
};

/**
 * Adopted by ace2_inner from the parent frame.
 */
export const adoptPluginsFromAncestorsOf = (frame: Window): void => {
  try {
    const parentDefs = (frame.parent as any)?.pluginDefs;
    if (parentDefs) {
      defs.loaded = parentDefs.loaded;
      defs.parts = parentDefs.parts;
      defs.plugins = parentDefs.plugins;
    }
    const parentPlugins = (frame.parent as any)?.plugins;
    if (parentPlugins?.baseURL) {
      baseURL = parentPlugins.baseURL;
    }
  } catch (error) {
    console.error('Could not adopt plugins from parent frame:', error);
  }
};
