// Gate re-keying: ConfigHub's ApplyGates/ApplyWarnings are keyed by the full
// `<space-slug>/<trigger-slug>/<function-name>` identifier, which is hard to
// know or type. To let FQL query gates by the human-meaningful TRIGGER slug
// (`gate['no-critical-cves']`), the transport also spreads each gate onto the
// row re-keyed by its trigger — OR-ing the booleans when a trigger has several
// gates. This is client-side only: ConfigHub can't match a map key's middle
// component server-side, so the exact `applyGates['<full/key>']` form remains
// the one that pushes down.

import type { Row } from '../fql';

/**
 * The trigger-slug component of a gate key. ConfigHub keys are
 * `<space-slug>/<trigger-slug>/<function-name>` (3-part) or a legacy
 * `<trigger-slug>/<function-name>` (2-part). Returns null for a key with no
 * `/` separator (nothing to re-key by).
 */
export function triggerOfGateKey(key: string): string | null {
  const parts = key.split('/');
  if (parts.length >= 3) return parts[1]; // space / trigger / function
  if (parts.length === 2) return parts[0]; // trigger / function (legacy)
  return null;
}

/**
 * Spread a gate map onto `row` under `prefix.<trigger-slug>`, OR-ing values so
 * the entry is true when ANY gate from that trigger is set. E.g. ApplyGates
 * `{ 'sec/no-critical-cves/vet': true }` → `row['gate.no-critical-cves'] = true`.
 */
export function spreadGatesByTrigger(
  row: Row,
  prefix: string,
  map: Record<string, boolean> | undefined,
): void {
  if (!map) return;
  for (const [key, value] of Object.entries(map)) {
    const trigger = triggerOfGateKey(key);
    if (trigger === null) continue;
    const field = `${prefix}.${trigger}`;
    row[field] = row[field] === true || value;
  }
}
