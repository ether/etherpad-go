export {};

declare global {
  interface EtherpadClientVars {
    automaticReconnectionTimeout?: number | string;
    mode?: string;
    padId?: string;
    randomVersionString: string;
    sessionRefreshInterval?: number | string;
    userId?: string | number;
    [key: string]: unknown;
  }

  interface Window {
    BroadcastSlider?: unknown;
    browser?: unknown;
    chat?: unknown;
    clientVars: EtherpadClientVars;
    customStart?: () => void;
    pad?: unknown;
    padeditbar?: unknown;
    padimpexp?: unknown;
    pluginDefs?: unknown;
    plugins?: {
      setBaseURL: (value: string) => void;
      update: (modules?: Map<string, unknown>) => Promise<void>;
    };
    require?: (moduleName: string) => unknown;
    _postPluginUpdateForTesting?: () => void;
    _postPluginUpdateForTestingDone?: boolean;
  }
}
