import { describe, expect, it } from 'vitest';

import { NEUTRAL, STATUS, categorical, sequential, statusColor } from './palette';

describe('statusColor', () => {
  it('reads "Not deployable" as neutral, not healthy', () => {
    // The bug this guards: /deployable/ matched the "good" rule, so base units
    // rendered the same green as applied-and-current ones.
    expect(statusColor('Not deployable', 'light')).toBe(NEUTRAL.light);
    expect(statusColor('Base / not deployable', 'light')).toBe(NEUTRAL.light);
  });

  it('reads "Never applied" as critical, not good', () => {
    expect(statusColor('Never applied', 'light')).toBe(STATUS.light.critical);
  });

  it('maps the apply states to distinct levels', () => {
    const states = ['Applied and current', 'Unapplied changes', 'Never applied', 'Not deployable'];
    const colors = states.map((s) => statusColor(s, 'light'));
    expect(new Set(colors).size).toBe(states.length);
  });

  it('gives residue buckets no hue', () => {
    expect(statusColor('Other', 'light')).toBe(NEUTRAL.light);
    expect(statusColor('(none)', 'dark')).toBe(NEUTRAL.dark);
  });

  it('falls back to neutral for anything that is not a state', () => {
    expect(statusColor('Deployment', 'light')).toBe(NEUTRAL.light);
  });
});

describe('sequential', () => {
  it('gives the largest value the most contrast against the surface', () => {
    // rank 0 is the largest value: dark on light, light on dark.
    expect(sequential(0, 5, 'light')).toBe('#184f95');
    expect(sequential(4, 5, 'light')).toBe('#86b6ef');
    expect(sequential(0, 5, 'dark')).toBe('#b7d3f6');
    expect(sequential(4, 5, 'dark')).toBe('#184f95');
  });

  it('keeps the least-contrast step visible for discrete marks', () => {
    // The ordinal floor: no step may recede into the surface, or a small bar vanishes.
    expect(sequential(9, 10, 'light')).not.toBe('#cde2fb');
  });

  it('handles a single category', () => {
    expect(sequential(0, 1, 'light')).toBeTruthy();
  });
});

describe('categorical', () => {
  it('assigns slots in fixed order', () => {
    expect(categorical(0, 'light')).toBe('#2a78d6');
    expect(categorical(1, 'light')).toBe('#eb6834');
  });

  it('does not wrap around past the token ceiling', () => {
    // Wrapping would give series 9 the same hue as series 1 and imply an identity
    // that is not there; callers fold the tail into "Other" instead.
    expect(categorical(20, 'light')).toBe(categorical(7, 'light'));
  });
});
