import { describe, expect, it } from 'vitest';

import { toDisplayName } from './displayName';

// The pattern ConfigHub enforces on DisplayName.
const PATTERN = /^[A-Za-z0-9]([\-_ .|A-Za-z0-9]*[A-Za-z0-9.!?])?$/;

describe('toDisplayName', () => {
  it('passes an already-legal title through', () => {
    expect(toDisplayName('Fleet Overview')).toBe('Fleet Overview');
  });

  it('strips characters the pattern rejects', () => {
    // The bug this guards: a title with parentheses failed the save with a raw regex
    // in the error body.
    const out = toDisplayName('Delivery Health (edited by configboard)');
    expect(out).toBe('Delivery Health edited by configboard');
    expect(PATTERN.test(out!)).toBe(true);
  });

  it('handles prose punctuation', () => {
    for (const title of [
      "Fleet Overview: what's live?",
      'Cost / spend, by team',
      '  leading and trailing  ',
      'Resources — by kind',
      '2026 review!',
    ]) {
      const out = toDisplayName(title);
      expect(out, title).toBeDefined();
      expect(PATTERN.test(out!), `${title} -> ${out}`).toBe(true);
    }
  });

  it('keeps a legal trailing terminator', () => {
    expect(toDisplayName('Is it live?')).toBe('Is it live?');
    expect(toDisplayName('Ship it!')).toBe('Ship it!');
  });

  it('returns undefined when nothing legal survives, so the field can be omitted', () => {
    expect(toDisplayName('***')).toBeUndefined();
    expect(toDisplayName('   ')).toBeUndefined();
    expect(toDisplayName('')).toBeUndefined();
    expect(toDisplayName('。。')).toBeUndefined();
  });

  it('accepts a single alphanumeric character', () => {
    expect(toDisplayName('A')).toBe('A');
    expect(PATTERN.test('A')).toBe(true);
  });

  it('caps length', () => {
    const out = toDisplayName('a'.repeat(500));
    expect(out!.length).toBeLessThanOrEqual(120);
    expect(PATTERN.test(out!)).toBe(true);
  });
});
