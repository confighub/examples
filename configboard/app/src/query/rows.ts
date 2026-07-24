// Normalizes API entities into flat `Row`s keyed by dimension id. Everything
// downstream (aggregation, charts, the table view) works on Rows and never touches
// an API shape.

import type {
  ExtendedRevisionRead,
  ExtendedSpaceRead,
  ExtendedTargetRead,
  ExtendedUnitRead,
} from '@confighub/rtk-query';

import type { Row, RowValue } from '../model/types';
import { hasId, realId } from './ids';

const HOURS = 1000 * 60 * 60;

/** ConfigHub renders "never" timestamps as the zero time rather than omitting them. */
const ZERO_TIME_PREFIX = '0001-01-01';

function time(value: string | undefined | null): string | null {
  if (!value || value.startsWith(ZERO_TIME_PREFIX)) return null;
  return value;
}

function count(map: Record<string, unknown> | undefined | null): number {
  return map ? Object.keys(map).length : 0;
}

function putMap(
  values: Record<string, RowValue>,
  prefix: string,
  map: Record<string, string> | undefined | null,
): void {
  if (!map) return;
  for (const [k, v] of Object.entries(map)) values[`${prefix}${k}`] = v;
}

/** Deep link to a Unit in the ConfigHub UI. */
export function unitHref(baseUrl: string, spaceId?: string, unitId?: string): string | undefined {
  const space = realId(spaceId);
  const unit = realId(unitId);
  if (!space || !unit) return undefined;
  return `${baseUrl.replace(/\/+$/, '')}/units/${space}/${unit}`;
}

export function unitRow(e: ExtendedUnitRead, baseUrl: string): Row {
  const u = e.Unit;
  const values: Record<string, RowValue> = {
    'Unit.Slug': u?.Slug ?? null,
    'Unit.DisplayName': u?.DisplayName || u?.Slug || null,
    'Unit.ToolchainType': u?.ToolchainType ?? null,
    'Unit.ProviderType': u?.ProviderType || '(none)',
    'Unit.HeadRevisionNum': u?.HeadRevisionNum ?? 0,
    'Unit.LiveRevisionNum': u?.LiveRevisionNum ?? 0,
    'Unit.UpstreamRevisionNum': u?.UpstreamRevisionNum ?? 0,
    'Unit.GateCount': count(u?.ApplyGates),
    'Unit.WarningCount': count(u?.ApplyWarnings),
    'Unit.UpdatedAt': time(u?.UpdatedAt),
    'Unit.LastChangeDescription': u?.LastChangeDescription ?? null,
    'Space.Slug': e.Space?.Slug ?? u?.SpaceSlug ?? null,
    'Target.Slug': e.Target?.Slug ?? null,
    'Target.ProviderType': e.Target?.ProviderType ?? null,
  };

  // A Unit with no Target is not a hole in the data: it is normally a base unit held
  // for cloning, or config that is not deployable on its own. Name the category.
  const targetId = realId(u?.TargetID);
  values['Unit.Deployable'] = targetId ? 'Deployable' : 'Base / not deployable';
  values['Unit.ApplyState'] = applyState(u?.HeadRevisionNum, u?.LiveRevisionNum, targetId);

  putMap(values, 'Unit.Labels.', u?.Labels);
  putMap(values, 'Unit.Values.', u?.Values);
  putMap(values, 'Space.Labels.', e.Space?.Labels);
  putMap(values, 'Target.Labels.', e.Target?.Labels);
  putMap(values, 'Target.Facts.', e.Target?.Facts);

  // View columns arrive as [{Name, Value}] when the query passed `view=`.
  for (const col of e.ViewColumns ?? []) {
    if (col.Name) values[`View.${col.Name}`] = col.Value ?? null;
  }

  return {
    id: realId(u?.UnitID) ?? `${e.Space?.Slug}/${u?.Slug}`,
    href: unitHref(baseUrl, u?.SpaceID, u?.UnitID),
    values,
  };
}

function applyState(head?: number, live?: number, targetId?: string): string {
  if (!hasId(targetId)) return 'Not deployable';
  if (!live) return 'Never applied';
  if ((head ?? 0) > live) return 'Unapplied changes';
  return 'Applied and current';
}

export function spaceRow(e: ExtendedSpaceRead): Row {
  const s = e.Space;
  const values: Record<string, RowValue> = {
    'Space.Slug': s?.Slug ?? null,
    'Space.DisplayName': s?.DisplayName || s?.Slug || null,
    'Space.TotalUnitCount': e.TotalUnitCount ?? 0,
    'Space.UnappliedUnitCount': e.UnappliedUnitCount ?? 0,
    'Space.UnapprovedUnitCount': e.UnapprovedUnitCount ?? 0,
    'Space.UnlinkedUnitCount': e.UnlinkedUnitCount ?? 0,
    'Space.GatedUnitCount': e.GatedUnitCount ?? 0,
    'Space.WarnedUnitCount': e.WarnedUnitCount ?? 0,
    'Space.UpgradableUnitCount': e.UpgradableUnitCount ?? 0,
    'Space.IncompleteApplyUnitCount': e.IncompleteApplyUnitCount ?? 0,
    'Space.TotalLinkCount': e.TotalLinkCount ?? 0,
    'Space.UpdatedAt': time(s?.UpdatedAt),
  };
  // Derived: the complement of "unapplied", which is what a meter wants.
  values['Space.CurrentUnitCount'] = (e.TotalUnitCount ?? 0) - (e.UnappliedUnitCount ?? 0);

  putMap(values, 'Space.Labels.', s?.Labels);
  return { id: s?.SpaceID ?? s?.Slug ?? '', values };
}

export function revisionRow(e: ExtendedRevisionRead): Row {
  const r = e.Revision;
  const createdAt = time(r?.CreatedAt);
  const liveAt = time(r?.LiveAt);
  const values: Record<string, RowValue> = {
    'Revision.Num': r?.RevisionNum ?? 0,
    'Revision.Source': r?.Source ?? null,
    'Revision.Description': r?.Description ?? null,
    'Revision.CreatedAt': createdAt,
    'Revision.LiveAt': liveAt,
    'Revision.Landed': liveAt ? 'Landed' : 'Not applied',
    'Space.Slug': e.Space?.Slug ?? r?.SpaceSlug ?? null,
    'Unit.Slug': e.Unit?.Slug ?? null,
  };

  values['Revision.LeadTimeHours'] =
    createdAt && liveAt
      ? Math.max(0, (Date.parse(liveAt) - Date.parse(createdAt)) / HOURS)
      : null;

  putMap(values, 'Space.Labels.', e.Space?.Labels);
  return { id: r?.RevisionID ?? '', values };
}

export function targetRow(e: ExtendedTargetRead): Row {
  const t = e.Target;
  const values: Record<string, RowValue> = {
    'Target.Slug': t?.Slug ?? null,
    'Target.DisplayName': t?.DisplayName || t?.Slug || null,
    'Target.ProviderType': t?.ProviderType ?? null,
    'Target.ToolchainType': t?.ToolchainType ?? null,
    'Target.KubernetesVersion': t?.Facts?.['Cluster.KubernetesVersion'] ?? null,
    'Target.ClusterName': t?.Facts?.['Cluster.Name'] ?? null,
    'Space.Slug': e.Space?.Slug ?? null,
  };
  putMap(values, 'Target.Labels.', t?.Labels);
  putMap(values, 'Target.Facts.', t?.Facts);
  return { id: t?.TargetID ?? t?.Slug ?? '', values };
}
