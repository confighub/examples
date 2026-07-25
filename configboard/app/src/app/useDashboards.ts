// Dashboard loading. Stored dashboards come from the `configboard` Space; the bundled
// documents are the seed, not the source of truth.
//
// Seeding is a write, so it never happens implicitly on load — the user is shown that
// the Space is empty and asked. Everything else here is read-only until called.

import { useCallback, useEffect, useMemo, useState } from 'react';

import { bundledDashboards } from '../dashboards';
import { appendPanel, parseDashboard } from '../model/parse';
import type { Dashboard, Panel } from '../model/types';
import { type StoredDashboard, useDashboardStorage } from '../storage/dashboards';
import { VIEW_SEEDS, useViewStorage } from '../storage/views';

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
  /** True when the Views that Tier-1 dashboards reference are not all present. */
  viewsMissing: boolean;
  /**
   * Bundled dashboards with no stored counterpart. A new starter dashboard shipped after
   * someone seeded must still be reachable — otherwise upgrading the app silently hides
   * the new work.
   */
  missingBundled: string[];
  reload: () => Promise<void>;
  seedBundled: () => Promise<void>;
  seedViews: () => Promise<void>;
  duplicate: (entry: DashboardEntry, slug: string, title: string) => Promise<void>;
  addPanel: (entry: DashboardEntry, panel: Panel) => Promise<void>;
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
  const views = useViewStorage();
  const [stored, setStored] = useState<StoredDashboard[] | null>(null);
  const [viewSlugs, setViewSlugs] = useState<string[] | null>(null);
  const [isLoading, setLoading] = useState(false);
  const [error, setError] = useState<string | undefined>();

  const reload = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const [dashboards, seeded] = await Promise.all([storage.list(), views.existing()]);
      setStored(dashboards);
      setViewSlugs(seeded);
    } catch (e) {
      setError(message(e));
      setStored([]);
      setViewSlugs([]);
    } finally {
      setLoading(false);
    }
  }, [storage, views]);

  const seedViews = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const spaceId = await storage.ensureSpace();
      await views.seed(spaceId);
      setViewSlugs(await views.existing());
    } catch (e) {
      setError(message(e));
    } finally {
      setLoading(false);
    }
  }, [storage, views]);

  useEffect(() => {
    if (!enabled) return;
    void reload();
    // reload is stable per storage instance; re-running on every render would loop.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [enabled]);

  /** Creates the bundled dashboards that are not stored yet. Existing ones are left alone. */
  const seedBundled = useCallback(async () => {
    setLoading(true);
    setError(undefined);
    try {
      const have = new Set((stored ?? []).map((s) => s.slug));
      for (const entry of bundledEntries()) {
        if (have.has(entry.dashboard.slug)) continue;
        await storage.create(entry.dashboard.slug, entry.yaml, entry.dashboard.title);
      }
      setStored(await storage.list());
    } catch (e) {
      setError(message(e));
    } finally {
      setLoading(false);
    }
  }, [storage, stored]);

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

  /** Appends a panel to a stored dashboard, leaving the rest of the document intact. */
  const addPanel = useCallback(
    async (entry: DashboardEntry, panel: Panel) => {
      if (!entry.stored) throw new Error('Save this dashboard to ConfigHub before adding panels.');
      await storage.save(entry.stored, appendPanel(entry.yaml, panel), `Add panel ${panel.title}`);
      setStored(await storage.list());
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

  const missingBundled = useMemo((): string[] => {
    if (stored === null || stored.length === 0) return [];
    const have = new Set(stored.map((s) => s.slug));
    return bundledEntries()
      .filter((e) => !have.has(e.dashboard.slug))
      .map((e) => e.dashboard.title);
  }, [stored]);

  return {
    entries,
    isLoading,
    isEmpty: stored !== null && stored.length === 0,
    viewsMissing: viewSlugs !== null && viewSlugs.length < VIEW_SEEDS.length,
    missingBundled,
    error,
    reload,
    seedBundled,
    seedViews,
    duplicate,
    addPanel,
    saveSource,
    remove,
  };
}
