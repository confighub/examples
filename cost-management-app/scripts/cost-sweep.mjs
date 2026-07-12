// Live cost sweep: two org-wide cub reads, the cost engine, one findings file.
// Read-only against ConfigHub. Writes data/cost-findings.json (deployment-local,
// gitignored: it contains live org content and belongs to this deployment).
import { spawnSync } from 'node:child_process';
import { readFile, writeFile } from 'node:fs/promises';
import { analyze, defaultRateCard } from '../src/cost-engine.mjs';

const OUT_PATH = 'data/cost-findings.json';
const RATE_CARD_PATH = 'data/rate-card.json';

const CONTAINER_CEL = "(r.kind == 'Deployment' || r.kind == 'StatefulSet') ? r.spec.template.spec.containers.map(c, {'Value': {'kind': r.kind, 'workload': r.metadata.name, 'namespace': has(r.metadata.namespace) ? r.metadata.namespace : '', 'container': c.name, 'replicas': has(r.spec.replicas) ? r.spec.replicas : 1, 'cpu_req': (has(c.resources) && has(c.resources.requests) && has(c.resources.requests.cpu)) ? string(c.resources.requests.cpu) : '', 'cpu_lim': (has(c.resources) && has(c.resources.limits) && has(c.resources.limits.cpu)) ? string(c.resources.limits.cpu) : '', 'mem_req': (has(c.resources) && has(c.resources.requests) && has(c.resources.requests.memory)) ? string(c.resources.requests.memory) : '', 'mem_lim': (has(c.resources) && has(c.resources.limits) && has(c.resources.limits.memory)) ? string(c.resources.limits.memory) : '', 'eph_lim': (has(c.resources) && has(c.resources.limits) && 'ephemeral-storage' in c.resources.limits) ? string(c.resources.limits['ephemeral-storage']) : ''}}) : []";

const CONTAINER_JQ = '. as $e | .Output[] | select(.Value != null) | {space: $e.SpaceSlug, unit: $e.UnitSlug} + .Value';
const UNITS_JQ = '.[] | {space: .Space.Slug, unit: .Unit.Slug, target: .Unit.TargetID, live: .Unit.LiveRevisionNum, head: .Unit.HeadRevisionNum, updated: .Unit.UpdatedAt}';

const CONTAINER_CMD = ['function', 'get', '--space', '*', 'get-cel', CONTAINER_CEL, '--show', 'output', '-o', `jq=${CONTAINER_JQ}`];
const UNITS_CMD = ['unit', 'list', '--space', '*', '--select', 'TargetID,LiveRevisionNum,HeadRevisionNum,UpdatedAt', '-o', `jq=${UNITS_JQ}`];

function runCub(cubArgs, label) {
  const result = spawnSync('cub', cubArgs, {encoding: 'utf8', maxBuffer: 64 * 1024 * 1024});
  if (result.error || result.status !== 0) {
    const detail = String(result.stderr || result.error || result.stdout || '').slice(0, 400);
    emit({
      verdict: 'BLOCK',
      reason: 'CONFIGHUB_READ_FAILED',
      status: 'COST_SWEEP_BLOCKED',
      step: label,
      detail,
      nextGate: 'Confirm cub auth status succeeds and the session can read the org, then rerun.',
    }, 1);
  }
  return result.stdout;
}

// jq's default output is a stream of pretty-printed objects; split on
// brace depth rather than assuming one-per-line.
export function splitJsonObjects(text) {
  const objects = [];
  let depth = 0;
  let start = -1;
  let inString = false;
  let escaped = false;
  for (let i = 0; i < text.length; i += 1) {
    const ch = text[i];
    if (inString) {
      if (escaped) escaped = false;
      else if (ch === '\\') escaped = true;
      else if (ch === '"') inString = false;
      continue;
    }
    if (ch === '"') { inString = true; continue; }
    if (ch === '{') {
      if (depth === 0) start = i;
      depth += 1;
    } else if (ch === '}') {
      depth -= 1;
      if (depth === 0 && start >= 0) {
        objects.push(JSON.parse(text.slice(start, i + 1)));
        start = -1;
      }
    }
  }
  return objects;
}

function emit(payload, exitCode) {
  console.log(JSON.stringify(payload, null, 2));
  process.exit(exitCode);
}

async function loadRateCard() {
  try {
    const card = JSON.parse(await readFile(RATE_CARD_PATH, 'utf8'));
    return {...defaultRateCard, ...card};
  } catch {
    return defaultRateCard;
  }
}

if (process.argv[1] && process.argv[1].endsWith('cost-sweep.mjs')) {
  const startedAt = new Date().toISOString();
  const containers = splitJsonObjects(runCub(CONTAINER_CMD, 'container-sweep'));
  const units = splitJsonObjects(runCub(UNITS_CMD, 'unit-bindings'));
  const rateCard = await loadRateCard();
  const report = analyze({containers, units}, rateCard, startedAt);
  report.provenance = {
    readOnly: true,
    commands: [
      `cub ${CONTAINER_CMD.slice(0, 5).join(' ')} <cel> --show output -o jq=<rows>`,
      `cub ${UNITS_CMD.slice(0, 5).join(' ')} ... -o jq=<rows>`,
    ],
    containerRows: containers.length,
    unitRows: units.length,
    startedAt,
    finishedAt: new Date().toISOString(),
  };
  report.schema = 'confighub.cost-findings.v0';
  await writeFile(OUT_PATH, `${JSON.stringify(report, null, 2)}\n`);
  emit({
    verdict: 'PASS',
    reason: 'COST_SWEEP_COMPLETE',
    status: 'COST_FINDINGS_WRITTEN',
    out: OUT_PATH,
    totals: report.totals,
    topFindings: report.findings.slice(0, 5).map(f => ({
      rule: f.rule,
      severity: f.severity,
      where: f.space ? `${f.space}${f.unit ? `/${f.unit}` : ''}` : f.workload,
      monthly: f.priced ? `${f.priced.monthly} ${f.priced.currency} (${f.priced.claim})` : null,
    })),
  }, 0);
}
