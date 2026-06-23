// Fleet snapshot loader. Discovers every Kubernetes/YAML Unit the user can view
// (optionally narrowed by the scope filters), extracts the Deployment resources
// server-side with get-resources, and builds one Workload per Unit: its images
// and the scanner's verdict (read from the Unit's annotations), joined with the
// gate/warning/revision metadata from the unit list.

import { useCallback, useMemo, useRef, useState } from 'react';

import { parse as parseYaml, parseAllDocuments } from 'yaml';

import { b64decodeUtf8 } from '../api/encoding';
import { fetchUnitDataText } from '../api/raw';
import { CvedbStatus, findingsByUnit, imagesOf, scanVerdict, Workload } from '../sec/model';
import {
  ExtendedUnitRead,
  FunctionInvocationsResponse,
  useInvokeFunctionsOnOrgMutation,
  useLazyListAllTargetsQuery,
  useLazyListAllUnitsQuery,
  useLazyListSpacesQuery,
} from '../sdk/confighubapi.gen';
import { FleetScope, loadScope } from './scope';

const K8S_UNITS_WHERE = "ToolchainType = 'Kubernetes/YAML'";
const REPORTS_WHERE = "ToolchainType = 'AppConfig/YAML'";
const DEPLOY_WHERE_DATA = "kind = 'Deployment'";
const DEPLOY_WHERE_RESOURCE = "ConfigHub.ResourceType = 'apps/v1/Deployment'";

interface RawResource {
  ResourceType?: string;
  ResourceName?: string;
  ResourceBody?: string;
}

function decodeResourceList(encoded: string): RawResource[] {
  try {
    const parsed: unknown = JSON.parse(b64decodeUtf8(encoded));
    return Array.isArray(parsed) ? (parsed as RawResource[]) : [];
  } catch {
    return [];
  }
}

// Canonical Spaces (base/policy) hold definitions, not deployed config. They
// stay flagged so dashboards can exclude them from "what's running" rollups.
const CANONICAL_VARIANTS = new Set(['base']);
const CANONICAL_DEMO_ROLES = new Set(['base', 'policy']);
function isCanonicalSpace(labels: Record<string, string> | undefined): boolean {
  return CANONICAL_VARIANTS.has(labels?.Variant ?? '') || CANONICAL_DEMO_ROLES.has(labels?.role ?? '');
}

export interface FleetSnapshot {
  /** One Workload per in-scope Deployment Unit. */
  workloads: Workload[];
  /** Workload by UnitID, for the unit detail page. */
  byUnit: Map<string, Workload>;
  /** In-scope unit metadata by UnitID (gates, warnings, revisions, target). */
  units: Map<string, ExtendedUnitRead>;
  /** Current CVE DB snapshot the fleet should be scanned against (null if unknown). */
  cvedb: CvedbStatus | null;
  scope: FleetScope;
  loadedAt: number;
}

export interface UseFleetSnapshotResult {
  snapshot: FleetSnapshot | null;
  isLoading: boolean;
  error: string | null;
  refresh: () => Promise<void>;
}

export function useFleetSnapshot(): UseFleetSnapshotResult {
  const [invoke] = useInvokeFunctionsOnOrgMutation();
  const [listUnits] = useLazyListAllUnitsQuery();
  const [listSpaces] = useLazyListSpacesQuery();
  const [listTargets] = useLazyListAllTargetsQuery();

  const [snapshot, setSnapshot] = useState<FleetSnapshot | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const callIdRef = useRef(0);

  const refresh = useCallback(async () => {
    const callId = ++callIdRef.current;
    setIsLoading(true);
    setError(null);
    const scope = loadScope();
    try {
      const [deployResult, unitsResult, spacesResult, targetsResult, reportsResult] = await Promise.all([
        invoke({
          where: K8S_UNITS_WHERE,
          whereData: DEPLOY_WHERE_DATA,
          functionInvocationsRequest: {
            WhereResource: DEPLOY_WHERE_RESOURCE,
            FunctionInvocations: [
              { FunctionName: 'get-resources', Arguments: [{ ParameterName: 'body', Value: 'json' }] },
            ],
          },
        }),
        listUnits({
          where: K8S_UNITS_WHERE,
          select:
            'UnitID,Slug,DisplayName,SpaceID,TargetID,Labels,ApplyGates,ApplyWarnings,' +
            'HeadRevisionNum,LiveRevisionNum,UpstreamRevisionNum,UpstreamUnitID,LastChangeDescription',
          include: 'SpaceID,TargetID',
        }),
        listSpaces({ where: scope.spaceWhere === '' ? undefined : scope.spaceWhere }),
        listTargets({ where: scope.targetWhere === '' ? undefined : scope.targetWhere, select: 'TargetID,Slug' }),
        // Scan-record Units (full findings, one multi-doc AppConfig/YAML Unit
        // per Space). Errors here are non-fatal — workloads still render with
        // their gate-signal verdict.
        listUnits({ where: REPORTS_WHERE, select: 'UnitID,Slug,SpaceID,Labels', include: 'SpaceID' }),
      ]);

      if (callId !== callIdRef.current) return; // superseded

      if ('error' in deployResult && deployResult.error) {
        setError('Deployment resource fetch failed');
        return;
      }
      if (unitsResult.error || spacesResult.error || targetsResult.error) {
        setError(
          spacesResult.error || targetsResult.error
            ? 'Scope filter query failed — check the filter expressions in Settings.'
            : 'Unit metadata fetch failed',
        );
        return;
      }

      const scopedSpaceIds = new Set(
        (spacesResult.data ?? []).map((s) => s.Space?.SpaceID).filter((id) => id !== undefined),
      );
      const scopedTargetIds = new Set(
        (targetsResult.data ?? []).map((t) => t.Target?.TargetID).filter((id) => id !== undefined),
      );

      // Scope rule: targeted units are in scope iff their Target matches; base
      // (untargeted) units iff their Space matches.
      const units = new Map<string, ExtendedUnitRead>();
      for (const eu of unitsResult.data ?? []) {
        const id = eu.Unit?.UnitID;
        if (id === undefined) continue;
        const targetId = eu.Unit?.TargetID;
        const inScope =
          targetId !== undefined && targetId !== null
            ? scopedTargetIds.has(targetId)
            : scopedSpaceIds.has(eu.Unit?.SpaceID ?? '');
        if (inScope) units.set(id, eu);
      }

      const workloads: Workload[] = [];
      for (const response of (deployResult.data as FunctionInvocationsResponse[]) ?? []) {
        if (!response.Success || !response.UnitID) continue;
        const eu = units.get(response.UnitID);
        if (!eu) continue; // out of scope

        const images: string[] = [];
        let verdict = {
          scanned: false,
          maxSeverity: 'UNKNOWN' as Workload['maxSeverity'],
          cveCount: 0,
          scannedAt: '',
          cvedbVersion: '',
        };
        for (const raw of decodeResourceList(response.Outputs?.['ResourceList'] ?? '')) {
          if (!raw.ResourceBody) continue;
          let doc: unknown;
          try {
            doc = JSON.parse(raw.ResourceBody);
          } catch {
            continue;
          }
          for (const img of imagesOf(doc)) if (!images.includes(img)) images.push(img);
          const v = scanVerdict(doc);
          if (v.scanned && !verdict.scanned) verdict = v; // first scanned Deployment's verdict
        }
        if (images.length === 0) continue;

        const target = eu.Target?.Slug;
        const space = response.SpaceSlug ?? eu.Space?.Slug ?? '';
        workloads.push({
          unitId: response.UnitID,
          unitSlug: response.UnitSlug ?? eu.Unit?.Slug ?? '',
          space,
          spaceId: response.SpaceID ?? eu.Unit?.SpaceID ?? '',
          target,
          cluster: target ?? space,
          env: eu.Space?.Labels?.env ?? eu.Unit?.Labels?.env,
          canonical: isCanonicalSpace(eu.Space?.Labels),
          images,
          scanned: verdict.scanned,
          maxSeverity: verdict.maxSeverity,
          cveCount: verdict.cveCount,
          scannedAt: verdict.scannedAt,
          cvedbVersion: verdict.cvedbVersion,
          findings: [],
          gates: Object.keys(eu.Unit?.ApplyGates ?? {}),
          warnings: Object.keys(eu.Unit?.ApplyWarnings ?? {}),
          headRevision: eu.Unit?.HeadRevisionNum,
        });
      }

      // From the in-scope AppConfig/YAML Units: attach findings from each Space's
      // sec-scan-record (a multi-doc YAML, one document per workload), and read
      // the single cvedb-status Unit (the current DB version) for staleness.
      const byKey = new Map(workloads.map((w) => [`${w.spaceId}/${w.unitSlug}`, w]));
      let cvedb: CvedbStatus | null = null;
      await Promise.all(
        (reportsResult.data ?? []).map(async (eu) => {
          const sid = eu.Unit?.SpaceID;
          const uid = eu.Unit?.UnitID;
          const role = eu.Unit?.Labels?.role;
          if (!sid || !uid || !scopedSpaceIds.has(sid)) return;
          try {
            const text = await fetchUnitDataText(sid, uid);
            if (role === 'scan-record') {
              const docs = parseAllDocuments(text).map((d) => d.toJS() as unknown);
              for (const [unit, findings] of findingsByUnit(docs)) {
                const wl = byKey.get(`${sid}/${unit}`);
                if (wl) wl.findings = findings;
              }
            } else if (role === 'cvedb-status') {
              const doc = parseYaml(text) as Record<string, unknown>;
              cvedb = {
                version: String(doc.cvedb_version ?? ''),
                advisories: Number(doc.advisories ?? 0),
                lastScanAt: String(doc.last_scan_at ?? ''),
              };
            }
          } catch {
            // ignore unreadable/unparseable report Units
          }
        }),
      );

      if (callId !== callIdRef.current) return; // superseded during report fetch

      const byUnit = new Map(workloads.map((w) => [w.unitId, w]));
      setSnapshot({ workloads, byUnit, units, cvedb, scope, loadedAt: Date.now() });
    } finally {
      if (callId === callIdRef.current) setIsLoading(false);
    }
  }, [invoke, listUnits, listSpaces, listTargets]);

  return useMemo(() => ({ snapshot, isLoading, error, refresh }), [snapshot, isLoading, error, refresh]);
}
