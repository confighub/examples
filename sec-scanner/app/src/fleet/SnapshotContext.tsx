// One fleet snapshot shared across pages: loaded lazily on first use,
// refreshed explicitly via the toolbar action.

import { ReactNode, createContext, useContext, useEffect } from 'react';

import { UseFleetSnapshotResult, useFleetSnapshot } from './snapshot';

const SnapshotContext = createContext<UseFleetSnapshotResult | null>(null);

export function SnapshotProvider({ children }: { children: ReactNode }) {
  const value = useFleetSnapshot();
  return <SnapshotContext.Provider value={value}>{children}</SnapshotContext.Provider>;
}

/**
 * Access the shared snapshot, kicking off the initial load if nobody has
 * yet. (The load is idempotent per provider instance.)
 */
export function useSnapshot(): UseFleetSnapshotResult {
  const ctx = useContext(SnapshotContext);
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
