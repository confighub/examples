// User-configurable analysis scope. Clusters correspond to Targets; Spaces
// scope base Units that have no Target. Both are selected with ConfigHub
// filter expressions (the same `where` syntax the CLI uses); empty means
// everything the user can view — the server already filters by view
// permission. Stored in the browser: scope is a per-user preference.

export interface FleetScope {
  /** Filter expression over Targets (e.g. "Slug LIKE 'prod-%'"). Empty = all. */
  targetWhere: string;
  /** Filter expression over Spaces, scoping untargeted base Units. Empty = all. */
  spaceWhere: string;
}

const STORAGE_KEY = 'rbac-manager-scope';

export function loadScope(): FleetScope {
  try {
    const raw = window.localStorage.getItem(STORAGE_KEY);
    if (raw !== null) {
      const parsed = JSON.parse(raw) as Partial<FleetScope>;
      return {
        targetWhere: typeof parsed.targetWhere === 'string' ? parsed.targetWhere : '',
        spaceWhere: typeof parsed.spaceWhere === 'string' ? parsed.spaceWhere : '',
      };
    }
  } catch {
    // Corrupt storage falls back to defaults.
  }
  return { targetWhere: '', spaceWhere: '' };
}

export function saveScope(scope: FleetScope): void {
  window.localStorage.setItem(STORAGE_KEY, JSON.stringify(scope));
}
