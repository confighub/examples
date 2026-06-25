// Live validation harness: runs the REAL app transport (fqlTransport) against a
// live ConfigHub over its REST API, so the full stack — planner pushdown,
// endpoint shapes, materializers — is exercised end to end, not mocked.
//
//   npx vite-node scripts/live.ts            # uses `cub auth get-token`
//   CONFIGHUB_TOKEN=… CONFIGHUB_URL=… npx vite-node scripts/live.ts
//
// It shims the two browser globals the transport uses (window.sessionStorage for
// the bearer token, and a fetch that rewrites same-origin /api paths to the real
// server) so the unmodified transport runs under Node. All queries are read-only.

import { execSync } from 'node:child_process';

const BASE = process.env.CONFIGHUB_URL ?? 'https://hub.confighub.com';
const TOKEN = (
  process.env.CONFIGHUB_TOKEN ?? execSync('cub auth get-token').toString()
).trim();

const store = new Map<string, string>([['confighub-token', TOKEN]]);
const sessionStorage = {
  getItem: (k: string) => store.get(k) ?? null,
  setItem: (k: string, v: string) => void store.set(k, v),
  removeItem: (k: string) => void store.delete(k),
};
(globalThis as unknown as { window: unknown }).window = { sessionStorage };
(globalThis as unknown as { sessionStorage: unknown }).sessionStorage = sessionStorage;

const realFetch = globalThis.fetch.bind(globalThis);
globalThis.fetch = ((input: unknown, init?: unknown) => {
  const url = typeof input === 'string' && input.startsWith('/') ? BASE + input : input;
  return realFetch(url as Parameters<typeof realFetch>[0], init as Parameters<typeof realFetch>[1]);
}) as typeof fetch;

const { runQuery, planQuery } = await import('../src/fql/index');
const { fqlTransport } = await import('../src/api/fqlTransport');

const QUERIES: string[] = [
  // ── fleet (real sec-demo data) ─────────────────────────────────────────────
  'SELECT slug, headRevisionNum, liveRevisionNum, lastAppliedRevisionNum FROM units WHERE space = ' + "'sec-demo-dev'",
  'SELECT slug, space FROM units WHERE headRevisionNum > liveRevisionNum',
  'SELECT cluster, COUNT(*) AS units FROM units GROUP BY cluster ORDER BY units DESC',
  "SELECT slug FROM spaces WHERE slug LIKE 'sec-demo-%'",
  // ── gates (the applyGates verification, live) ───────────────────────────────
  "SELECT slug, space FROM units WHERE gate['no-critical-cves'] = true",
  "SELECT slug FROM units WHERE applyGates['sec-demo-policy/no-critical-cves/vet-celexpr'] = true",
  // ── resources: kinds, raw paths, scanner annotation ─────────────────────────
  "SELECT unit, kind, name FROM resources WHERE space = 'sec-demo-dev' AND kind = 'Deployment'",
  "SELECT unit, `spec.template.spec.containers.*.image` AS image FROM resources WHERE space = 'sec-demo-dev'",
  "SELECT unit, metadata.annotations['sec-scanner.confighub.com/max-severity'] AS sev FROM resources WHERE space = 'sec-demo-dev' AND metadata.annotations['sec-scanner.confighub.com/max-severity'] = 'CRITICAL'",
  "SELECT unit, `spec.template.spec.containers.*.image` AS image FROM resources WHERE space = 'sec-demo-dev' AND `spec.template.spec.containers.*.image` LIKE '%:latest'",
  // ── revisions + time-travel ────────────────────────────────────────────────
  "SELECT unit, revisionNum, source, description FROM revisions WHERE space = 'sec-demo-dev' ORDER BY revisionNum DESC LIMIT 5",
  "SELECT unit, kind, name, `spec.template.spec.containers.*.image` AS image FROM resources WHERE space = 'sec-demo-dev' AND revision = 'live'",
  // numeric time-travel: legacy-frontend's first revision (proves the data-blob fetch)
  "SELECT unit, revision, `spec.template.spec.containers.*.image` AS image FROM resources WHERE unit = 'legacy-frontend' AND revision = 1",
  "SELECT unit, revision, `spec.template.spec.containers.*.image` AS image FROM resources WHERE unit = 'legacy-frontend' AND revision = 'head'",
  // ── RBAC (no rbac-demo data here → expect 0 rows, proves the pipeline runs) ──
  "SELECT subject, cluster, role FROM grants WHERE space = 'sec-demo-dev' AND verb = 'get' AND resource = 'secrets'",
  "SELECT cluster, name FROM roles WHERE space = 'sec-demo-dev'",
  "SELECT cluster, name, orphaned FROM bindings WHERE space = 'sec-demo-dev'",
  "SELECT analyzer, COUNT(*) AS n FROM rbac_findings WHERE space = 'sec-demo-dev' GROUP BY analyzer",
];

let ok = 0;
let err = 0;
for (const q of QUERIES) {
  const oneLine = q.replace(/\s+/g, ' ');
  try {
    const plan = planQuery(q);
    const fetchSummary = plan.fetches
      .map(
        (f) =>
          Object.entries(f)
            .map(([k, v]) => `${k}=${typeof v === 'object' ? JSON.stringify(v) : v}`)
            .join(' ') || '(no-pushdown)',
      )
      .join(' || ');
    const res = await runQuery(q, fqlTransport);
    ok++;
    console.log(
      `\n✅ ${oneLine}\n   plan: [${fetchSummary}]\n   rows=${res.stats.resultRows} fetched=${res.stats.fetchedRows} calls=${res.stats.fetches}`,
    );
    for (const r of res.rows.slice(0, 4)) console.log('      ', JSON.stringify(r));
  } catch (e) {
    err++;
    console.log(`\n❌ ${oneLine}\n   ${(e as Error).message ?? e}`);
  }
}
console.log(`\n──────\n${ok} ok, ${err} failed (of ${QUERIES.length})`);
