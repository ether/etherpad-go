import {describe, expect, it} from 'vitest';
import {readdirSync, readFileSync} from 'node:fs';
import {join} from 'node:path';
import {THEME_CSS} from './sheetTheme';

describe('sheet theme tokens', () => {
  const dir = __dirname;
  const used = new Set<string>();
  for (const f of readdirSync(dir)) {
    if (!f.endsWith('.ts') || f.endsWith('.test.ts') || f === 'sheetTheme.ts') continue;
    for (const m of readFileSync(join(dir, f), 'utf8').matchAll(/var\((--sheet-[a-z-]+),/g)) used.add(m[1]);
  }

  it('the sheet uses the tokens', () => {
    expect(used.size).toBeGreaterThan(5);
  });

  it('every token the sheet reads is set by the etherpad-go theme', () => {
    const light = THEME_CSS.slice(0, THEME_CSS.indexOf('@media'));
    for (const token of used) expect(light, token).toContain(`${token}:`);
  });

  it('every token outside the theme has a fallback (classic look)', () => {
    for (const f of readdirSync(dir)) {
      if (!f.endsWith('.ts') || f.endsWith('.test.ts') || f === 'sheetTheme.ts') continue;
      const src = readFileSync(join(dir, f), 'utf8');
      expect(src.match(/var\(--sheet-[a-z-]+\)/g) ?? [], f).toEqual([]);
    }
  });
});
