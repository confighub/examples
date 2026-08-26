// User-configurable analysis scope. Clusters correspond to Targets; Spaces scope base
// Units that have no Target. Both are selected with ConfigHub filter expressions (the
// same `where` syntax the CLI uses); empty means everything the user can view — the
// server already filters by view permission.
//
// Scope is a per-user preference, so it lives in the browser. Each app owns its own
// storage key: two example apps open in the same browser analyze different things.

export interface FleetScope {
  /** Filter expression over Targets (e.g. "Slug LIKE 'prod-%'"). Empty = all. */
  targetWhere: string;
  /** Filter expression over Spaces, scoping untargeted base Units. Empty = all. */
  spaceWhere: string;
}

export const EMPTY_SCOPE: FleetScope = { targetWhere: '', spaceWhere: '' };

export interface ScopeStore {
  load: () => FleetScope;
  save: (scope: FleetScope) => void;
}

/** A scope store backed by localStorage under the given key. */
export function createScopeStore(storageKey: string): ScopeStore {
  return {
    load(): FleetScope {
      try {
        const raw = window.localStorage.getItem(storageKey);
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
      return { ...EMPTY_SCOPE };
    },
    save(scope: FleetScope): void {
      window.localStorage.setItem(storageKey, JSON.stringify(scope));
    },
  };
}
