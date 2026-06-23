// Severity vocabulary shared across the console. Mirrors the scanner's buckets
// (CRITICAL ≥ 9.0, HIGH ≥ 7.0, MEDIUM ≥ 4.0, LOW > 0); NONE = scanned, clean;
// UNKNOWN = not yet scanned (no max-severity annotation on the Unit).

export type Severity = 'CRITICAL' | 'HIGH' | 'MEDIUM' | 'LOW' | 'NONE' | 'UNKNOWN';

export const SEVERITIES: Severity[] = ['CRITICAL', 'HIGH', 'MEDIUM', 'LOW', 'NONE', 'UNKNOWN'];

const RANK: Record<Severity, number> = {
  UNKNOWN: -1,
  NONE: 0,
  LOW: 1,
  MEDIUM: 2,
  HIGH: 3,
  CRITICAL: 4,
};

export function severityRank(s: Severity): number {
  return RANK[s] ?? -1;
}

export function isSeverity(s: string): s is Severity {
  return s in RANK;
}

/** Highest severity in a list (UNKNOWN if empty). */
export function maxSeverity(list: Severity[]): Severity {
  return list.reduce<Severity>((acc, s) => (severityRank(s) > severityRank(acc) ? s : acc), 'UNKNOWN');
}

/** MUI Chip/`color` mapping. HIGH has no dedicated palette slot, so it reuses
 * warning with a filled variant to read as more severe than MEDIUM. */
export function severityColor(
  s: Severity,
): 'error' | 'warning' | 'info' | 'success' | 'default' {
  switch (s) {
    case 'CRITICAL':
      return 'error';
    case 'HIGH':
      return 'warning';
    case 'MEDIUM':
      return 'warning';
    case 'LOW':
      return 'info';
    case 'NONE':
      return 'success';
    default:
      return 'default';
  }
}

export function severityLabel(s: Severity): string {
  return s === 'UNKNOWN' ? 'unscanned' : s.toLowerCase();
}
