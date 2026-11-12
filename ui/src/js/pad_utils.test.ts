import {afterEach, beforeAll, describe, expect, it} from 'vitest';
import padutils from './pad_utils';

// etherpad-lite: src/tests/backend-new/specs/pad_utils.ts
describe('warnDeprecated', () => {
  const {warnDeprecatedFlags, warnDeprecated} = padutils;
  let logger: any;

  beforeAll(() => { logger = warnDeprecatedFlags.logger; });

  afterEach(() => {
    warnDeprecatedFlags.logger = logger;
    delete warnDeprecatedFlags._rl; // Reset the rate limiter.
  });

  it('rate limited', () => {
    let got = 0;
    warnDeprecatedFlags.logger = {warn: () => ++got};
    warnDeprecated(); // Initialise the rate limiter.
    const {period} = warnDeprecatedFlags._rl!;
    got = 0;
    const testCases = [[0, 1], [0, 1], [period - 1, 1], [period, 2]];
    for (const [now, want] of testCases) { // In a loop so that the stack trace is the same.
      warnDeprecatedFlags._rl!.now = () => now;
      warnDeprecated();
      expect(got).toEqual(want);
    }
    warnDeprecated(); // A different stack trace is a different warning.
    expect(got).toEqual(testCases[testCases.length - 1][1] + 1);
  });
});
