// One fleet snapshot per app: loaded on demand, refreshed explicitly, never polled.
//
// The load is imperative rather than a query hook because it is several requests whose
// results are only meaningful together, and because a stale in-flight load must not
// overwrite a newer one — the call-id guard below is the whole reason this is not a
// useEffect. `build` is the app's part: it turns the scope into whatever that app
// analyzes.

import { useCallback, useMemo, useRef, useState } from 'react';

import type { FleetScope, ScopeStore } from './scope';
import { ScopeQueryError } from './units';

/** What every snapshot carries, whatever the app adds to it. */
export interface SnapshotBase {
  scope: FleetScope;
  loadedAt: number;
}

export interface UseFleetSnapshotResult<T> {
  snapshot: (T & SnapshotBase) | null;
  isLoading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
}

/**
 * @param store  where the app keeps its scope
 * @param build  loads the app's analysis for one scope; may throw to report an error
 */
export function useFleetSnapshot<T>(
  store: ScopeStore,
  build: (scope: FleetScope) => Promise<T>,
): UseFleetSnapshotResult<T> {
  const [snapshot, setSnapshot] = useState<(T & SnapshotBase) | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const callIdRef = useRef(0);

  const refresh = useCallback(async () => {
    const callId = ++callIdRef.current;
    setIsLoading(true);
    setError(null);
    const scope = store.load();
    try {
      const built = await build(scope);
      if (callId !== callIdRef.current) return; // a newer refresh superseded us
      setSnapshot({ ...built, scope, loadedAt: Date.now() });
    } catch (e) {
      if (callId !== callIdRef.current) return;
      setError(
        e instanceof ScopeQueryError
          ? e.message
          : e instanceof Error
            ? e.message
            : 'Fleet snapshot failed',
      );
    } finally {
      if (callId === callIdRef.current) setIsLoading(false);
    }
  }, [store, build]);

  return useMemo(
    () => ({ snapshot, isLoading, error, refresh }),
    [snapshot, isLoading, error, refresh],
  );
}
