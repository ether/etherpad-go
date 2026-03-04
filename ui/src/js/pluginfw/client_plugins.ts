import defs from './plugin_defs';
import * as pluginUtils from './shared';

type HookParts = Array<Record<string, unknown> & {
  client_hooks?: Record<string, string>;
}>;

type PluginDefinitionsResponse = {
  plugins: Record<string, unknown>;
  parts: HookParts;
};

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

const loadPluginScript = (pluginPath: string): Promise<void> => new Promise((resolve) => {
  if (pluginUtils.isModuleRegistered?.(pluginPath)) {
    resolve();
    return;
  }

  const url = `${baseURL}static/plugins/${pluginPath}.js?v=${window.clientVars.randomVersionString}`;
  const script = document.createElement('script');
  script.src = url;
  script.type = 'text/javascript';
  script.onload = () => {
    const moduleRegistry = window as unknown as Record<string, unknown>;
    if (moduleRegistry[pluginPath] != null) {
      pluginUtils.registerPluginModule(pluginPath, moduleRegistry[pluginPath]);
    }
    resolve();
  };
  script.onerror = (err) => {
    console.warn(`Failed to load plugin script: ${url}`, err);
    resolve();
  };
  document.head.appendChild(script);
});

const getPluginPaths = (parts: HookParts): string[] => {
  const paths = new Set<string>();
  for (const part of parts) {
    if (part.client_hooks == null) continue;
    for (const hookFnPath of Object.values(part.client_hooks)) {
      const modulePath = hookFnPath.split(':')[0];
      if (!modulePath.startsWith('ep_etherpad-lite/')) {
        paths.add(modulePath);
      }
    }
  }
  return [...paths];
};

const fetchPluginDefinitions = async (): Promise<PluginDefinitionsResponse> => {
  const response = await fetch(
      `${baseURL}pluginfw/plugin-definitions.json?v=${window.clientVars.randomVersionString}`);
  if (!response.ok) {
    throw new Error(`Failed to load plugin definitions (${response.status})`);
  }
  return await response.json() as PluginDefinitionsResponse;
};

export const update = async (modules?: Map<string, unknown>): Promise<void> => {
  const data = await fetchPluginDefinitions();
  defs.plugins = data.plugins;
  defs.parts = data.parts;

  const pluginPaths = getPluginPaths(data.parts);
  await Promise.all(pluginPaths.map(loadPluginScript));

  defs.hooks = pluginUtils.extractHooks(defs.parts, 'client_hooks', null, modules);
  defs.loaded = true;
};

export const adoptPluginsFromAncestorsOf = (frame: Window): void => {
  let parentRequire: ((moduleName: string) => unknown) | null = null;
  try {
    while ((frame = frame.parent)) {
      if (typeof (frame.require) !== 'undefined') {
        parentRequire = frame.require as (moduleName: string) => unknown;
        break;
      }
    }
  } catch (error) {
    console.error(error);
  }

  if (parentRequire == null) throw new Error('Parent plugins could not be found.');

  const ancestorPluginDefs = parentRequire('ep_etherpad-lite/static/js/pluginfw/plugin_defs') as typeof defs;
  defs.hooks = ancestorPluginDefs.hooks;
  defs.loaded = ancestorPluginDefs.loaded;
  defs.parts = ancestorPluginDefs.parts;
  defs.plugins = ancestorPluginDefs.plugins;

  const ancestorPlugins = parentRequire('ep_etherpad-lite/static/js/pluginfw/client_plugins') as {
    baseURL: string;
  };
  baseURL = ancestorPlugins.baseURL;
};
