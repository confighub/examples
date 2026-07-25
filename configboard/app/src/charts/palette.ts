// The chart palette. Categorical hues are assigned in fixed slot order and never
// cycled — a 9th series folds into "Other" instead of getting a generated hue, which
// is why `aggregate()` has a topN fold. Both modes are selected sets stepped for
// their own surface, not an inversion of each other.

export type Mode = 'light' | 'dark';

/** Fixed slot order. Adjacent pairs are the ones that must stay distinguishable. */
const CATEGORICAL: Record<Mode, string[]> = {
  light: ['#2a78d6', '#eb6834', '#1baf7a', '#eda100', '#e87ba4', '#008300', '#4a3aa7', '#e34948'],
  dark: ['#3987e5', '#d95926', '#199e70', '#c98500', '#d55181', '#008300', '#9085e9', '#e66767'],
};

/**
 * Single-hue ramp for magnitude, ordered least -> most contrast against the mode's
 * surface. These are the *ordinal*-safe steps: discrete marks (bars, cells) must stay
 * visible, so the step nearest the surface still clears 2:1 — on light that means
 * starting no lighter than step 250, on dark no darker than step 600. A continuous
 * heatmap could use the full 100–700 range, where near-zero is allowed to recede.
 */
const SEQUENTIAL: Record<Mode, string[]> = {
  light: ['#86b6ef', '#6da7ec', '#5598e7', '#3987e5', '#2a78d6', '#256abf', '#1c5cab', '#184f95'],
  dark: ['#184f95', '#1c5cab', '#256abf', '#2a78d6', '#3987e5', '#5598e7', '#86b6ef', '#b7d3f6'],
};

/** Reserved for state. Never reused as "series 4". */
export const STATUS: Record<Mode, Record<StatusLevel, string>> = {
  light: { good: '#008300', warning: '#eda100', serious: '#eb6834', critical: '#e34948' },
  dark: { good: '#008300', warning: '#c98500', serious: '#d95926', critical: '#e66767' },
};

export type StatusLevel = 'good' | 'warning' | 'serious' | 'critical';

export const NEUTRAL: Record<Mode, string> = { light: '#8f8e88', dark: '#6f6e69' };
export const SURFACE: Record<Mode, string> = { light: '#fcfcfb', dark: '#1a1a19' };
export const GRID: Record<Mode, string> = { light: '#e6e5e1', dark: '#2f2f2c' };

export const OTHER_KEY = 'Other';
export const NONE_KEY = '(none)';

/**
 * Categorical colour for a series. Colour follows the entity, not its rank: the index
 * comes from the series' position in the full, stable key list, so filtering the chart
 * down to fewer series never repaints the survivors.
 */
export function categorical(index: number, mode: Mode): string {
  const slots = CATEGORICAL[mode];
  if (index < 0) return NEUTRAL[mode];
  // Past the token ceiling the caller should have folded to "Other"; hold the last
  // slot rather than wrapping around to slot 1 and implying a false identity.
  return slots[Math.min(index, slots.length - 1)];
}

/**
 * Sequential step for a value's rank, where rank 0 is the *largest* value. More is
 * darker on light, lighter on dark — in both cases more contrast against the surface,
 * which is what "more" has to look like.
 */
export function sequential(rank: number, count: number, mode: Mode): string {
  const ramp = SEQUENTIAL[mode];
  if (count <= 1) return ramp[ramp.length - 3];
  const t = 1 - rank / (count - 1);
  const idx = Math.round(t * (ramp.length - 1));
  return ramp[Math.max(0, Math.min(ramp.length - 1, idx))];
}

/** Residue and empty buckets are never a hue — they read as absence. */
export function keyColor(key: string, index: number, mode: Mode, role: 'categorical' | 'sequential', count: number): string {
  if (key === OTHER_KEY || key === NONE_KEY) return NEUTRAL[mode];
  return role === 'sequential' ? sequential(index, count, mode) : categorical(index, mode);
}

/**
 * Category name -> status level, tested in severity order. Order is load-bearing:
 * "Not deployable" must not match the "deployable" rule and read as healthy, and
 * "Never applied" must not match the "applied" rule. Anything that is not a health
 * state at all — a base unit, a missing value — is neutral, not good.
 */
const STATUS_WORDS: { match: RegExp; level: StatusLevel | 'neutral' }[] = [
  { match: /^(not deployable|base\b|not applicable|n\/a)/i, level: 'neutral' },
  { match: /(never applied|fail|error|gated|blocked|critical|denied)/i, level: 'critical' },
  { match: /(unapproved|incomplete|stale|serious)/i, level: 'serious' },
  { match: /(unapplied|pending|warn|behind|upgradable|drift)/i, level: 'warning' },
  { match: /(applied and current|landed|completed|passing|healthy|current|good)/i, level: 'good' },
];

/**
 * Maps a category name to a status colour. Status is always paired with the label
 * itself in the legend and table, so the state never rests on colour alone.
 */
export function statusColor(key: string, mode: Mode): string {
  if (key === OTHER_KEY || key === NONE_KEY) return NEUTRAL[mode];
  for (const { match, level } of STATUS_WORDS) {
    if (match.test(key)) return level === 'neutral' ? NEUTRAL[mode] : STATUS[mode][level];
  }
  return NEUTRAL[mode];
}

/** Emphasis: one series in the accent hue, everything else recessive. */
export function emphasisColor(key: string, emphasized: string | undefined, mode: Mode): string {
  return key === emphasized ? CATEGORICAL[mode][0] : NEUTRAL[mode];
}
