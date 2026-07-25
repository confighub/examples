import { describe, expect, it } from 'vitest';

import { hasId, realId } from './ids';

describe('realId', () => {
  it('passes a real id through', () => {
    expect(realId('7e621917-66a2-49f3-b4a1-f1b432cf0ae2')).toBe(
      '7e621917-66a2-49f3-b4a1-f1b432cf0ae2',
    );
  });

  it('treats the zero UUID as absent', () => {
    expect(realId('00000000-0000-0000-0000-000000000000')).toBeUndefined();
  });

  it('treats null, undefined, and empty as absent', () => {
    expect(realId(null)).toBeUndefined();
    expect(realId(undefined)).toBeUndefined();
    expect(realId('')).toBeUndefined();
  });
});

describe('hasId', () => {
  it('is false for the zero UUID', () => {
    // The whole point: a plain truthiness check on the raw string is true here, which
    // is how "no Target" became a Target that could not be resolved.
    expect(Boolean('00000000-0000-0000-0000-000000000000')).toBe(true);
    expect(hasId('00000000-0000-0000-0000-000000000000')).toBe(false);
  });

  it('is true for a real id', () => {
    expect(hasId('afc82365-0222-41e5-9573-510662d7aef2')).toBe(true);
  });
});
