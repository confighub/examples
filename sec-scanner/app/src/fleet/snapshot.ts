// Fleet snapshot loader. Discovers every Kubernetes/YAML Unit in scope, extracts the
// Deployment resources server-side with get-resources, and builds one Workload per Unit:
// its images and the scanner's verdict (read from the Unit's annotations), joined with
// the gate/warning/revision metadata from the unit list.
//
// The scan records are a second, separate population: one multi-doc AppConfig/YAML Unit
// per Space holding the full findings, plus a single cvedb-status Unit. Which kind a Unit
// is comes from its `role` label, so those take two requests — the list for the labels and
// the bulk data endpoint for the documents, joined on UnitID. Two requests for every
// record in scope, rather than one per Space.

import { confighub, getResources, listUnitData, resourceDocs } from '@confighub/examples-webkit/api';
import {
  createSnapshotContext,
  isCanonicalSpace,
  loadScopedUnits,
  type ExtendedUnit,
  type FleetScope,
} from '@confighub/examples-webkit/fleet';
import { parse as parseYaml, parseAllDocuments } from 'yaml';

import {
  findingsByUnit,
  imagesOf,
  scanVerdict,
  type CvedbStatus,
  type Workload,
} from '../sec/model';
import { scopeStore } from './scope';

const K8S_UNITS_WHERE = "ToolchainType = 'Kubernetes/YAML'";
const REPORTS_WHERE = "ToolchainType = 'AppConfig/YAML'";
const DEPLOY_WHERE_DATA = "kind = 'Deployment'";
const DEPLOY_WHERE_RESOURCE = "ConfigHub.ResourceType = 'apps/v1/Deployment'";

export interface FleetSnapshot {
  /** One Workload per in-scope Deployment Unit. */
  workloads: Workload[];
  /** Workload by UnitID, for the unit detail page. */
  byUnit: Map<string, Workload>;
  /** In-scope unit metadata by UnitID (gates, warnings, revisions, target). */
  units: Map<string, ExtendedUnit>;
  /** Current CVE DB snapshot the fleet should be scanned against (null if unknown). */
  cvedb: CvedbStatus | null;
}

async function build(scope: FleetScope): Promise<FleetSnapshot> {
  const [deployResponses, scoped, records] = await Promise.all([
    getResources({
      where: K8S_UNITS_WHERE,
      whereData: DEPLOY_WHERE_DATA,
      whereResource: DEPLOY_WHERE_RESOURCE,
    }),
    loadScopedUnits(scope, { where: K8S_UNITS_WHERE }),
    // Non-fatal: workloads still render with their gate-signal verdict if this fails.
    loadRecords().catch(() => []),
  ]);

  const workloads: Workload[] = [];
  for (const response of deployResponses) {
    if (!response.Success || !response.UnitID) continue;
    const eu = scoped.units.get(response.UnitID);
    if (!eu) continue; // out of scope

    const images: string[] = [];
    let verdict = {
      scanned: false,
      maxSeverity: 'UNKNOWN' as Workload['maxSeverity'],
      cveCount: 0,
      scannedAt: '',
      cvedbVersion: '',
    };
    for (const { doc } of resourceDocs(response)) {
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

  // Attach each Space's sec-scan-record findings to its workloads, and read the single
  // cvedb-status Unit for staleness.
  const byKey = new Map(workloads.map((w) => [`${w.spaceId}/${w.unitSlug}`, w]));
  let cvedb: CvedbStatus | null = null;
  for (const record of records) {
    const sid = record.spaceId;
    if (!scoped.spaceIds.has(sid)) continue;
    try {
      if (record.role === 'scan-record') {
        const docs = parseAllDocuments(record.data).map((d) => d.toJS() as unknown);
        for (const [unit, findings] of findingsByUnit(docs)) {
          const wl = byKey.get(`${sid}/${unit}`);
          if (wl) wl.findings = findings;
        }
      } else if (record.role === 'cvedb-status') {
        const doc = parseYaml(record.data) as Record<string, unknown>;
        cvedb = {
          version: String(doc.cvedb_version ?? ''),
          advisories: Number(doc.advisories ?? 0),
          lastScanAt: String(doc.last_scan_at ?? ''),
        };
      }
    } catch {
      // A record that does not parse is a record this app cannot use.
    }
  }

  return { workloads, byUnit: new Map(workloads.map((w) => [w.unitId, w])), units: scoped.units, cvedb };
}

interface ScanRecord {
  spaceId: string;
  /** `scan-record` or `cvedb-status`, from the Unit's label. */
  role: string;
  data: string;
}

/**
 * The scan-record Units with their configurations. `/unit_data` carries the documents but
 * not the labels, and the label is what says which kind of record a Unit is — so the list
 * supplies the roles and the two are joined on UnitID.
 */
async function loadRecords(): Promise<ScanRecord[]> {
  const [listed, documents] = await Promise.all([
    confighub().GET('/unit', {
      params: { query: { where: REPORTS_WHERE, select: 'UnitID,SpaceID,Labels' } },
    }),
    listUnitData({ where: REPORTS_WHERE }),
  ]);

  const roles = new Map<string, string>();
  for (const eu of listed.data ?? []) {
    const id = eu.Unit?.UnitID;
    const role = eu.Unit?.Labels?.role;
    if (id !== undefined && role !== undefined) roles.set(id, role);
  }

  const records: ScanRecord[] = [];
  for (const d of documents) {
    const role = roles.get(d.UnitID ?? '');
    if (role === undefined || d.SpaceID === undefined || d.Data === undefined) continue;
    records.push({ spaceId: d.SpaceID, role, data: d.Data });
  }
  return records;
}

export const { SnapshotProvider, useSnapshot } = createSnapshotContext<FleetSnapshot>(
  scopeStore,
  build,
);
