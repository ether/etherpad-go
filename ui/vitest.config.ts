import { defineConfig } from 'vitest/config';

export default defineConfig({
  test: {
    environment: 'node',
    // Plugin logic without DOM lives next to the plugin.
    include: ['src/**/*.test.ts', '../plugins/*/static/js/**/*.test.ts'],
  },
});
