// The security model the console renders. A Workload is one Kubernetes/YAML
// Unit containing a Deployment: its container images, the gate-signal verdict
// the scanner wrote back as annotations (max-severity + cve-count), and the
// full findings — which live in the Space's AppConfig/YAML "sec-scan-record"
// Unit (a multi-document YAML, one document per workload).
//
// Nothing is recomputed in the browser — the scanner (scanner/secscan) owns CVE
// matching and records its result in ConfigHub as data; this module just reads it.

import { isSeverity, maxSeverity, Severity } from './severity';

export const ANNO_MAX_SEVERITY = 'sec-scanner.confighub.com/max-severity';
export const ANNO_CVE_COUNT = 'sec-scanner.confighub.com/cve-count';
export const ANNO_SCANNED_AT = 'sec-scanner.confighub.com/scanned-at';
export const ANNO_CVEDB_VERSION = 'sec-scanner.confighub.com/cvedb-version';

/** The cvedb-status Unit: which CVE DB snapshot the fleet should be scanned against. */
export interface CvedbStatus {
  version: string; // latest import timestamp; the staleness reference
  advisories: number;
  lastScanAt: string;
}

/** One vulnerable (package, advisory) pair — shape matches the report YAML. */
export interface Finding {
  advisory: string;
  severity: Severity;
  cvss_score?: number;
  package: string;
  version: string;
  fixed_version?: string;
}

export interface Workload {
  unitId: string;
  unitSlug: string;
  space: string;
  spaceId: string;
  target?: string;
  /** Cluster key: Target slug if bound, else Space slug. */
  cluster: string;
  env?: string;
  canonical: boolean;
  images: string[];
  /** True once the scanner has written a verdict onto the Unit. */
  scanned: boolean;
  maxSeverity: Severity;
  cveCount: number;
  /** When the workload was scanned, and the CVE DB version it was scanned against. */
  scannedAt: string;
  cvedbVersion: string;
  /** Full findings, loaded from the Space's AppConfig/YAML sec-scan-record Unit. */
  findings: Finding[];
  /** Apply Gate keys (blocking) and ApplyWarning keys (advisory). */
  gates: string[];
  warnings: string[];
  headRevision?: number;
}

interface ContainerLike {
  image?: string;
}
interface DeploymentDoc {
  kind?: string;
  metadata?: { annotations?: Record<string, string> };
  spec?: { template?: { spec?: { containers?: ContainerLike[]; initContainers?: ContainerLike[] } } };
}

/** Extract container images from a parsed Deployment document. */
export function imagesOf(doc: unknown): string[] {
  const d = doc as DeploymentDoc;
  const spec = d?.spec?.template?.spec;
  const all = [...(spec?.containers ?? []), ...(spec?.initContainers ?? [])];
  const seen = new Set<string>();
  const out: string[] = [];
  for (const c of all) {
    if (typeof c?.image === 'string' && c.image !== '' && !seen.has(c.image)) {
      seen.add(c.image);
      out.push(c.image);
    }
  }
  return out;
}

/** Read the scanner's gate-signal verdict off a Deployment's annotations.
 * (The full findings live in the Space's sec-scan-record Unit, not here.) */
export function scanVerdict(doc: unknown): {
  scanned: boolean;
  maxSeverity: Severity;
  cveCount: number;
  scannedAt: string;
  cvedbVersion: string;
} {
  const annos = (doc as DeploymentDoc)?.metadata?.annotations ?? {};
  const sevRaw = annos[ANNO_MAX_SEVERITY];
  const scanned = typeof sevRaw === 'string' && sevRaw !== '';
  const sev: Severity = scanned && isSeverity(sevRaw) ? (sevRaw as Severity) : 'UNKNOWN';
  const count = Number.parseInt(annos[ANNO_CVE_COUNT] ?? '', 10);
  return {
    scanned,
    maxSeverity: sev,
    cveCount: Number.isFinite(count) ? count : 0,
    scannedAt: annos[ANNO_SCANNED_AT] ?? '',
    cvedbVersion: annos[ANNO_CVEDB_VERSION] ?? '',
  };
}

/** A scanned workload is stale if it was judged by an older CVE DB than the
 * current one (the cvedb-status version). */
export function isStale(w: Workload, status: CvedbStatus | null): boolean {
  return (
    !!status?.version && w.scanned && !!w.cvedbVersion && w.cvedbVersion < status.version
  );
}

/** Pull the findings array out of one parsed scan-record document. */
export function findingsFromReport(doc: unknown): Finding[] {
  const list = (doc as { findings?: unknown })?.findings;
  if (!Array.isArray(list)) return [];
  return list
    .filter((f): f is Finding => typeof (f as Finding)?.advisory === 'string')
    .map((f) => ({ ...f, severity: isSeverity(f.severity) ? f.severity : ('UNKNOWN' as Severity) }));
}

/** Findings keyed by workload slug, parsed from a Space's sec-scan-record — a
 * multi-document YAML where each document is one workload's report (with a
 * leading `unit:` field). */
export function findingsByUnit(docs: unknown[]): Map<string, Finding[]> {
  const out = new Map<string, Finding[]>();
  for (const doc of docs) {
    const unit = (doc as { unit?: unknown })?.unit;
    if (typeof unit === 'string' && unit !== '') out.set(unit, findingsFromReport(doc));
  }
  return out;
}

/** Roll one severity bucket count per severity across a set of workloads. */
export function severityHistogram(workloads: Workload[]): Record<Severity, number> {
  const hist: Record<Severity, number> = {
    CRITICAL: 0,
    HIGH: 0,
    MEDIUM: 0,
    LOW: 0,
    NONE: 0,
    UNKNOWN: 0,
  };
  for (const w of workloads) hist[w.maxSeverity] += 1;
  return hist;
}

export { maxSeverity };
