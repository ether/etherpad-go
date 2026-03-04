// This module contains processed plugin definitions. The data structures in this file are set by
// plugins.js (server) or client_plugins.js (client).
type PluginDefsState = {
  hooks: Record<string, unknown>;
  loaded: boolean;
  parts: unknown[];
  plugins: Record<string, unknown>;
};

const defs: PluginDefsState = {
  hooks: {},
  loaded: false,
  parts: [],
  plugins: {},
};

export default defs;
