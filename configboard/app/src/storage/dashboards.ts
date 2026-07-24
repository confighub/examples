// Dashboards as data. Each dashboard is one AppConfig/YAML Unit in the app's own
// `configboard` Space; the YAML body is the Dashboard document. The Space is created
// on first use.
//
// This is the only part of configboard that writes anything, and it writes only its own
// documents — never a Unit it did not create, never anything in a Space it does not own.
//
// Reads go through the raw /data text endpoint (Unit.Data is base64 over the JSON API;
// /data returns the plain YAML). Writes set Unit.Data base64-encoded.

import {
  useCreateSpaceMutation,
  useCreateUnitMutation,
  useDeleteUnitMutation,
  useLazyListAllUnitsQuery,
  useLazyListSpacesQuery,
  usePatchUnitMutation,
} from '@confighub/rtk-query';
import { useCallback } from 'react';

import { b64encodeUtf8 } from '../api/encoding';
import { fetchUnitDataText } from '../api/raw';
import { parseDashboard } from '../model/parse';
import type { Dashboard } from '../model/types';
import { toDisplayName } from './displayName';

/** Slug of the Space that holds this app's dashboard units. */
export const STORAGE_SPACE_SLUG = 'configboard';
/** Label marking units this app owns. */
export const APP_LABEL = 'configboard';

const OWNED_WHERE = `Labels.app = '${APP_LABEL}'`;
const UNIT_FIELDS = 'UnitID,Slug,SpaceID,Version,HeadRevisionNum,Labels,DisplayName,UpdatedAt';

export interface StoredDashboard {
  unitId: string;
  spaceId: string;
  slug: string;
  /** Optimistic-concurrency version of the unit entity — not a config revision. */
  version: number;
  /** Head config revision of the unit: how many times the document has changed. */
  headRevision: number;
  /** The document as stored, verbatim — what the source editor shows. */
  yaml: string;
  dashboard: Dashboard;
  /** Validation problems in the stored document, surfaced rather than swallowed. */
  errors: string[];
}

export interface DashboardStorage {
  ensureSpace: () => Promise<string>;
  list: () => Promise<StoredDashboard[]>;
  create: (slug: string, yaml: string, title: string) => Promise<void>;
  save: (entry: StoredDashboard, yaml: string, changeDesc: string) => Promise<void>;
  remove: (entry: StoredDashboard) => Promise<void>;
}

function unwrap<T>(result: { data?: T; error?: unknown }, what: string): T {
  if (result.error || result.data === undefined) {
    const detail =
      typeof result.error === 'object' && result.error !== null
        ? JSON.stringify((result.error as { data?: unknown }).data ?? result.error).slice(0, 200)
        : '';
    throw new Error(`${what} failed${detail ? `: ${detail}` : ''}`);
  }
  return result.data;
}

export function useDashboardStorage(): DashboardStorage {
  const [listSpaces] = useLazyListSpacesQuery();
  const [createSpace] = useCreateSpaceMutation();
  const [listUnits] = useLazyListAllUnitsQuery();
  const [createUnit] = useCreateUnitMutation();
  const [patchUnit] = usePatchUnitMutation();
  const [deleteUnit] = useDeleteUnitMutation();

  /** Looks up the storage Space without creating it. Returns undefined if absent. */
  const findSpace = useCallback(async (): Promise<string | undefined> => {
    const spaces = unwrap(
      await listSpaces({ where: `Slug = '${STORAGE_SPACE_SLUG}'`, select: 'SpaceID,Slug' }),
      'list spaces',
    );
    return spaces.find((s) => s.Space?.Slug === STORAGE_SPACE_SLUG)?.Space?.SpaceID;
  }, [listSpaces]);

  const ensureSpace = useCallback(async (): Promise<string> => {
    const existing = await findSpace();
    if (existing) return existing;

    const created = unwrap(
      await createSpace({
        allowExists: 'true',
        space: {
          Slug: STORAGE_SPACE_SLUG,
          DisplayName: 'configboard',
          Labels: { app: APP_LABEL },
        },
      }),
      'create space',
    );
    if (!created.SpaceID) throw new Error('create space returned no SpaceID');
    return created.SpaceID;
  }, [findSpace, createSpace]);

  const list = useCallback(async (): Promise<StoredDashboard[]> => {
    // Reading must not write. Before anyone has saved a dashboard the Space does not
    // exist, and creating it on load would mean opening the app mutated the org.
    const spaceId = await findSpace();
    if (!spaceId) return [];

    const units = unwrap(
      await listUnits({
        where: `SpaceID = '${spaceId}' AND ${OWNED_WHERE}`,
        select: UNIT_FIELDS,
      }),
      'list dashboard units',
    );

    const entries = await Promise.all(
      units.map(async (eu): Promise<StoredDashboard | null> => {
        const u = eu.Unit;
        if (!u?.UnitID || !u.SpaceID || !u.Slug) return null;
        const yaml = await fetchUnitDataText(u.SpaceID, u.UnitID);
        const { dashboard, errors } = parseDashboard(yaml);
        if (!dashboard) {
          // A stored document that no longer parses is reported, not hidden: the unit
          // exists and the user needs to be able to open and fix it.
          return {
            unitId: u.UnitID,
            spaceId: u.SpaceID,
            slug: u.Slug,
            version: u.Version ?? 0,
            headRevision: u.HeadRevisionNum ?? 0,
            yaml,
            dashboard: {
              apiVersion: '',
              kind: 'Dashboard',
              slug: u.Slug,
              title: u.DisplayName || u.Slug,
              panels: [],
            },
            errors,
          };
        }
        return {
          unitId: u.UnitID,
          spaceId: u.SpaceID,
          slug: u.Slug,
          version: u.Version ?? 0,
          headRevision: u.HeadRevisionNum ?? 0,
          yaml,
          dashboard,
          errors,
        };
      }),
    );

    return entries
      .filter((e): e is StoredDashboard => e !== null)
      .sort((a, b) => a.dashboard.title.localeCompare(b.dashboard.title));
  }, [findSpace, listUnits]);

  const create = useCallback(
    async (slug: string, yaml: string, title: string): Promise<void> => {
      const spaceId = await ensureSpace();
      const displayName = toDisplayName(title);
      unwrap(
        await createUnit({
          spaceId,
          unit: {
            Slug: slug,
            ...(displayName ? { DisplayName: displayName } : {}),
            ToolchainType: 'AppConfig/YAML',
            Labels: { app: APP_LABEL },
            Data: b64encodeUtf8(yaml),
            LastChangeDescription: `Create dashboard ${title}`,
          },
        }),
        'create dashboard unit',
      );
    },
    [ensureSpace, createUnit],
  );

  const save = useCallback(
    async (entry: StoredDashboard, yaml: string, changeDesc: string): Promise<void> => {
      const { dashboard } = parseDashboard(yaml);
      // The title is the document's; DisplayName is ConfigHub's constrained label. Send
      // it only when something legal survives, so a prose title never blocks a save.
      const displayName = dashboard ? toDisplayName(dashboard.title) : undefined;
      // Merge-patch (not PUT): PUT-updating Data runs an optimistic check on a revision
      // UUID and 409s when it is not supplied; merge-patch applies to head directly.
      unwrap(
        await patchUnit({
          spaceId: entry.spaceId,
          unitId: entry.unitId,
          body: {
            ...(displayName ? { DisplayName: displayName } : {}),
            Data: b64encodeUtf8(yaml),
            LastChangeDescription: changeDesc,
          },
        }),
        'patch dashboard unit',
      );
    },
    [patchUnit],
  );

  const remove = useCallback(
    async (entry: StoredDashboard): Promise<void> => {
      unwrap(
        await deleteUnit({ spaceId: entry.spaceId, unitId: entry.unitId }),
        'delete dashboard unit',
      );
    },
    [deleteUnit],
  );

  return { ensureSpace, list, create, save, remove };
}
