// Dashboard loading. Stored dashboards come from the `configboard` Space; the bundled
// documents are the seed, not the source of truth.
//
// Seeding is a write, so it never happens implicitly on load — the user is shown that
// the Space is empty and asked. Everything else here is read-only until called.

import { useCallback, useEffect, useMemo, useState } from 'react';

import { bundledDashboards } from '../dashboards';
import { parseDashboard } from '../model/parse';
import type { Dashboard } from '../model/types';
import { type StoredDashboard, useDashboardStorage } from '../storage/dashboards';

export interface DashboardEntry {
  dashboard: Dashboard;
  errors: string[];
  /** The document text, for the source editor. */
  yaml: string;
  /** Present when the dashboard is stored in ConfigHub; absent for a bundled one. */
  stored?: StoredDashboard;
}

export interface DashboardsState {
  entries: DashboardEntry[];
  /** True while the initial load is in flight. */
  isLoading: boolean;
  /** True when the storage Space holds no dashboards yet. */
  isEmpty: boolean;
  error?: string;
  reload: () => Promise<void>;
  seedBundled: () => Promise<void>;
  duplicate: (entry: DashboardEntry, slug: string, title: string) => Promise<void>;
  saveSource: (entry: DashboardEntry, yaml: string) => Promise<void>;
  remove: (entry: DashboardEntry) => Promise<void>;
}

/** The bundled documents, parsed, as fallback and as seed material. */
function bundledEntries(): DashboardEntry[] {
  return bundledDashboards().map(({ dashboard, errors, yaml }) => ({
    dashboard,
    errors,
    yaml,
  }));
}

function message(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

export function useDashboards(enabled: boolean): DashboardsState {
  const storage = useDashboardStorage();
  const [stored, setStored] = useState<StoredDashboard[] | null>(null);
  const [isLoading, setLoading] = useState(false);
  const [error, setError] = useState<string | undefined>();

  const reload = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      setStored(await storage.list());
    } catch (e) {
      setError(message(e));
      setStored([]);
    } finally {
      setLoading(false);
    }
  }, [storage]);

  useEffect(() => {
    if (!enabled) return;
    void reload();
    // reload is stable per storage instance; re-running on every render would loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);

  const seedBundled = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      for (const entry of bundledEntries()) {
        await storage.create(entry.dashboard.slug, entry.yaml, entry.dashboard.title);
      }
      setStored(await storage.list());
    } catch (e) {
      setError(message(e));
    } finally {
      setLoading(false);
    }
  }, [storage]);

  const duplicate = useCallback(
    async (entry: DashboardEntry, slug: string, title: string) => {
      setError(undefined);
      try {
        // Rewrite slug and title in the copy so the document matches its Unit.
        const { dashboard } = parseDashboard(entry.yaml);
        const yaml = entry.yaml
          .replace(/^slug:.*$/m, `slug: ${slug}`)
          .replace(/^title:.*$/m, `title: ${title}`);
        if (!dashboard) throw new Error('source document does not parse');
        await storage.create(slug, yaml, title);
        setStored(await storage.list());
      } catch (e) {
        setError(message(e));
      }
    },
    [storage],
  );

  const saveSource = useCallback(
    async (entry: DashboardEntry, yaml: string) => {
      setError(undefined);
      if (!entry.stored) {
        setError('This dashboard is bundled, not stored — save it to ConfigHub first.');
        return;
      }
      try {
        await storage.save(entry.stored, yaml, `Edit dashboard ${entry.dashboard.title}`);
        setStored(await storage.list());
      } catch (e) {
        setError(message(e));
      }
    },
    [storage],
  );

  const remove = useCallback(
    async (entry: DashboardEntry) => {
      setError(undefined);
      if (!entry.stored) return;
      try {
        await storage.remove(entry.stored);
        setStored(await storage.list());
      } catch (e) {
        setError(message(e));
      }
    },
    [storage],
  );

  const entries = useMemo((): DashboardEntry[] => {
    if (stored === null) return [];
    if (stored.length === 0) {
      // Nothing stored yet: show the bundled documents so the app is useful before
      // anyone decides to write anything.
      return bundledEntries();
    }
    return stored.map((s) => ({
      dashboard: s.dashboard,
      errors: s.errors,
      yaml: s.yaml,
      stored: s,
    }));
  }, [stored]);

  return {
    entries,
    isLoading,
    isEmpty: stored !== null && stored.length === 0,
    error,
    reload,
    seedBundled,
    duplicate,
    saveSource,
    remove,
  };
}
