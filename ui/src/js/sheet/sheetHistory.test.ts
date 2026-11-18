import { describe, expect, it } from 'vitest';
import { GROUP_GAP_MS, groupRevisions } from './sheetHistory';

const min = 60 * 1000;

describe('history versions', () => {
  it('groups changes until a pause, newest first, with their authors', () => {
    const versions = groupRevisions([
      { rev: 1, timestamp: 0, author: 'a' },
      { rev: 2, timestamp: 1 * min, author: 'b' },
      { rev: 3, timestamp: 2 * min, author: 'a' },
      { rev: 4, timestamp: 30 * min, author: 'b' },
      { rev: 5, timestamp: 30 * min + GROUP_GAP_MS - 1 },
      { rev: 6, timestamp: 60 * min, author: 'c' },
    ]);
    expect(versions).toEqual([
      { rev: 6, from: 6, start: 60 * min, end: 60 * min, authors: ['c'] },
      { rev: 5, from: 4, start: 30 * min, end: 30 * min + GROUP_GAP_MS - 1, authors: ['b'] },
      { rev: 3, from: 1, start: 0, end: 2 * min, authors: ['a', 'b'] },
    ]);
  });

  it('starts a new version when the clock jumps back', () => {
    const versions = groupRevisions([
      { rev: 1, timestamp: 60 * min },
      { rev: 2, timestamp: 0 },
    ]);
    expect(versions.map((v) => v.rev)).toEqual([2, 1]);
    expect(groupRevisions([])).toEqual([]);
  });
});
