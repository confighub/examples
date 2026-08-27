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
    'Unit.LastReleasedRevisionNum': u?.LastReleasedRevisionNum ?? 0,
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
  values['Unit.ReleaseState'] = releaseState(u?.HeadRevisionNum, u?.LastReleasedRevisionNum, targetId);

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

function releaseState(head?: number, released?: number, targetId?: string): string {
  if (!hasId(targetId)) return 'Not deployable';
  if (!released) return 'Never released';
  if ((head ?? 0) > released) return 'Unreleased changes';
  return 'Released and current';
}

export function spaceRow(e: ExtendedSpaceRead): Row {
  const s = e.Space;
  const values: Record<string, RowValue> = {
    'Space.Slug': s?.Slug ?? null,
    'Space.DisplayName': s?.DisplayName || s?.Slug || null,
    'Space.TotalUnitCount': e.TotalUnitCount ?? 0,
    'Space.UnreleasedUnitCount': e.UnreleasedUnitCount ?? 0,
    'Space.UnapprovedUnitCount': e.UnapprovedUnitCount ?? 0,
    'Space.UnlinkedUnitCount': e.UnlinkedUnitCount ?? 0,
    'Space.GatedUnitCount': e.GatedUnitCount ?? 0,
    'Space.WarnedUnitCount': e.WarnedUnitCount ?? 0,
    'Space.UpgradableUnitCount': e.UpgradableUnitCount ?? 0,
    'Space.TotalLinkCount': e.TotalLinkCount ?? 0,
    'Space.UpdatedAt': time(s?.UpdatedAt),
  };
  // Derived: the complement of "unreleased", which is what a meter wants.
  values['Space.CurrentUnitCount'] = (e.TotalUnitCount ?? 0) - (e.UnreleasedUnitCount ?? 0);

  putMap(values, 'Space.Labels.', s?.Labels);
  return { id: s?.SpaceID ?? s?.Slug ?? '', values };
}

export function revisionRow(e: ExtendedRevisionRead): Row {
  const r = e.Revision;
  const createdAt = time(r?.CreatedAt);
  const values: Record<string, RowValue> = {
    'Revision.Num': r?.RevisionNum ?? 0,
    'Revision.Source': r?.Source ?? null,
    'Revision.Description': r?.Description ?? null,
    'Revision.CreatedAt': createdAt,
    // Whether a Release has bundled this Revision. Revision.LiveAt used to answer a
    // stronger question -- when a bridge put it on a cluster -- and is gone with the
    // bridge. Membership in a Release is what ConfigHub still knows, and it is a claim
    // about publication, not about what the cluster is running.
    'Revision.Released': count(r?.Releases) > 0 ? 'Released' : 'Not released',
    'Space.Slug': e.Space?.Slug ?? r?.SpaceSlug ?? null,
    'Unit.Slug': e.Unit?.Slug ?? null,
  };


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

/**
 * One row per failing check on a Unit. The map key is
 * `<policy-space>/<trigger-slug>/<function>`, which is what lets a findings chart group
 * by the guardrail that fired rather than by the Unit that failed it.
 */
export function findingRows(e: ExtendedUnitRead, baseUrl: string): Row[] {
  const base = unitRow(e, baseUrl);
  const out: Row[] = [];

  const add = (kind: 'Gate' | 'Warning', key: string) => {
    // Older or hand-set entries may not carry all three segments; keep whatever is there
    // rather than dropping the finding.
    const parts = key.split('/');
    const [policySpace, trigger, fn] =
      parts.length >= 3
        ? [parts[0], parts.slice(1, -1).join('/'), parts[parts.length - 1]]
        : ['', parts[0] ?? key, parts[1] ?? ''];

    out.push({
      id: `${base.id}#${kind}#${key}`,
      href: base.href,
      values: {
        ...base.values,
        'Finding.Kind': kind,
        'Finding.Key': key,
        'Finding.Trigger': trigger || key,
        'Finding.PolicySpace': policySpace || null,
        'Finding.Function': fn || null,
      },
    });
  };

  for (const key of Object.keys(e.Unit?.ApplyGates ?? {})) add('Gate', key);
  for (const key of Object.keys(e.Unit?.ApplyWarnings ?? {})) add('Warning', key);
  return out;
}
