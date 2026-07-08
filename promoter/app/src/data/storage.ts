// Persistence for promotion workflows. Each workflow is one AppConfig/YAML
// Unit in the app's own `promoter` Space; the YAML body is the Workflow
// document (see model/workflow.ts). The Space is created on first use.
//
// Reads go through the raw /data text endpoint (Unit.Data is base64 over the
// JSON API; the /data endpoint returns the plain YAML). Writes set Unit.Data
// directly, base64-encoded.

import { useCallback } from 'react';

import { fetchUnitDataText } from '../api/raw';
import { b64encodeUtf8 } from '../api/encoding';
import { parseWorkflow, serializeWorkflow, Workflow } from '../model/workflow';
import {
  useCreateSpaceMutation,
  useCreateUnitMutation,
  useDeleteUnitMutation,
  useLazyListAllUnitsQuery,
  useLazyListSpacesQuery,
  usePatchUnitMutation,
} from '@confighub/rtk-query';

/** Slug of the Space that holds this app's workflow units. */
export const STORAGE_SPACE_SLUG = 'promoter';
/** Label marking units this app owns. */
export const APP_LABEL = 'promoter';
const WORKFLOWS_WHERE = `Labels.app = '${APP_LABEL}'`;

export interface WorkflowEntry {
  unitId: string;
  spaceId: string;
  slug: string;
  /** Optimistic-concurrency version of the unit, needed for updates. */
  version: number;
  workflow: Workflow;
}

export interface Storage {
  ensureSpace: () => Promise<string>;
  listWorkflows: () => Promise<WorkflowEntry[]>;
  loadWorkflow: (slug: string) => Promise<WorkflowEntry | null>;
  createWorkflow: (slug: string, wf: Workflow) => Promise<void>;
  saveWorkflow: (entry: WorkflowEntry, wf: Workflow, changeDesc: string) => Promise<void>;
  deleteWorkflow: (entry: WorkflowEntry) => Promise<void>;
}

function unwrap<T>(result: { data?: T; error?: unknown }, what: string): T {
  if (result.error || result.data === undefined) {
    throw new Error(`${what} failed`);
  }
  return result.data;
}

export function useStorage(): Storage {
  const [listSpaces] = useLazyListSpacesQuery();
  const [createSpace] = useCreateSpaceMutation();
  const [listUnits] = useLazyListAllUnitsQuery();
  const [createUnit] = useCreateUnitMutation();
  const [patchUnit] = usePatchUnitMutation();
  const [deleteUnit] = useDeleteUnitMutation();

  const ensureSpace = useCallback(async (): Promise<string> => {
    const spaces = unwrap(
      await listSpaces({ where: `Slug = '${STORAGE_SPACE_SLUG}'`, select: 'SpaceID,Slug' }),
      'list spaces',
    );
    const existing = spaces.find((s) => s.Space?.Slug === STORAGE_SPACE_SLUG)?.Space;
    if (existing?.SpaceID) return existing.SpaceID;

    const created = unwrap(
      await createSpace({
        allowExists: 'true',
        space: {
          Slug: STORAGE_SPACE_SLUG,
          DisplayName: 'Promoter',
          Labels: { app: APP_LABEL },
        },
      }),
      'create space',
    );
    if (!created.SpaceID) throw new Error('create space returned no SpaceID');
    return created.SpaceID;
  }, [listSpaces, createSpace]);

  const listWorkflows = useCallback(async (): Promise<WorkflowEntry[]> => {
    const spaceId = await ensureSpace();
    const units = unwrap(
      await listUnits({
        where: `SpaceID = '${spaceId}' AND ${WORKFLOWS_WHERE}`,
        select: 'UnitID,Slug,SpaceID,Version,Labels',
      }),
      'list workflow units',
    );
    const entries = await Promise.all(
      units.map(async (eu): Promise<WorkflowEntry | null> => {
        const u = eu.Unit;
        if (!u?.UnitID || !u.SpaceID || !u.Slug) return null;
        const yaml = await fetchUnitDataText(u.SpaceID, u.UnitID);
        return {
          unitId: u.UnitID,
          spaceId: u.SpaceID,
          slug: u.Slug,
          version: u.Version ?? 0,
          workflow: parseWorkflow(yaml, u.Slug),
        };
      }),
    );
    return entries.filter((e): e is WorkflowEntry => e !== null);
  }, [ensureSpace, listUnits]);

  const loadWorkflow = useCallback(
    async (slug: string): Promise<WorkflowEntry | null> => {
      const spaceId = await ensureSpace();
      const units = unwrap(
        await listUnits({
          where: `SpaceID = '${spaceId}' AND Slug = '${slug}' AND ${WORKFLOWS_WHERE}`,
          select: 'UnitID,Slug,SpaceID,Version,Labels',
        }),
        'load workflow unit',
      );
      const u = units[0]?.Unit;
      if (!u?.UnitID || !u.SpaceID || !u.Slug) return null;
      const yaml = await fetchUnitDataText(u.SpaceID, u.UnitID);
      return {
        unitId: u.UnitID,
        spaceId: u.SpaceID,
        slug: u.Slug,
        version: u.Version ?? 0,
        workflow: parseWorkflow(yaml, u.Slug),
      };
    },
    [ensureSpace, listUnits],
  );

  const createWorkflow = useCallback(
    async (slug: string, wf: Workflow): Promise<void> => {
      const spaceId = await ensureSpace();
      unwrap(
        await createUnit({
          spaceId,
          unit: {
            Slug: slug,
            DisplayName: wf.name,
            ToolchainType: 'AppConfig/YAML',
            Labels: { app: APP_LABEL },
            Data: b64encodeUtf8(serializeWorkflow(wf)),
            LastChangeDescription: `Create promotion workflow ${wf.name}`,
          },
        }),
        'create workflow unit',
      );
    },
    [ensureSpace, createUnit],
  );

  const saveWorkflow = useCallback(
    async (entry: WorkflowEntry, wf: Workflow, changeDesc: string): Promise<void> => {
      // Merge-patch (not PUT): PUT-updating Data runs an optimistic check on a
      // revision UUID and 409s ("Config data changed") when not supplied; the
      // merge-patch path applies the data change to head directly.
      unwrap(
        await patchUnit({
          spaceId: entry.spaceId,
          unitId: entry.unitId,
          body: {
            DisplayName: wf.name,
            Data: b64encodeUtf8(serializeWorkflow(wf)),
            LastChangeDescription: changeDesc,
          },
        }),
        'patch workflow unit',
      );
    },
    [patchUnit],
  );

  const deleteWorkflow = useCallback(
    async (entry: WorkflowEntry): Promise<void> => {
      unwrap(await deleteUnit({ spaceId: entry.spaceId, unitId: entry.unitId }), 'delete workflow unit');
    },
    [deleteUnit],
  );

  return { ensureSpace, listWorkflows, loadWorkflow, createWorkflow, saveWorkflow, deleteWorkflow };
}
