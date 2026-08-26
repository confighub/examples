// The snapshot every page of an app shares: loaded lazily on first use, refreshed
// explicitly from the toolbar.

import { createContext, useContext, useEffect, type ReactNode } from 'react';

import type { FleetScope, ScopeStore } from './scope';
import { useFleetSnapshot, type UseFleetSnapshotResult } from './useSnapshot';

/**
 * Build a provider and hook pair for one app's snapshot type. Called once at module
 * scope, so the context identity is stable.
 */
export function createSnapshotContext<T>(
  store: ScopeStore,
  build: (scope: FleetScope) => Promise<T>,
) {
  const Context = createContext<UseFleetSnapshotResult<T> | null>(null);

  function SnapshotProvider({ children }: { children: ReactNode }) {
    const value = useFleetSnapshot<T>(store, build);
    return <Context.Provider value={value}>{children}</Context.Provider>;
  }

  /** Access the shared snapshot, kicking off the initial load if nobody has yet. */
  function useSnapshot(): UseFleetSnapshotResult<T> {
    const ctx = useContext(Context);
    if (!ctx) throw new Error('useSnapshot must be used within SnapshotProvider');
    const { snapshot, isLoading, refresh } = ctx;
    useEffect(() => {
      if (snapshot === null && !isLoading) {
        void refresh();
      }
      // Initial load only; refresh identity is stable per provider.
      // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);
    return ctx;
  }

  return { SnapshotProvider, useSnapshot };
}
