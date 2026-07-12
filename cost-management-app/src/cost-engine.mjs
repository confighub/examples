// Cost analysis over ConfigHub Unit data. Pure functions: rows in, findings out.
//
// Honesty rules, enforced in code, not prose:
// - Requests drive node provisioning, so only request-backed numbers are
//   priced as cost. Limits are exposure, never savings.
// - A Unit with no Target binding has no runtime cost claim: its numbers are
//   reported as configured-cost-only and excluded from savings totals.
// - Every priced figure carries the rate-card basis string so no number
//   travels without its assumptions.

export const NIL_UUID = '00000000-0000-0000-0000-000000000000';

export const defaultRateCard = {
  currency: 'USD',
  cpuPerCoreMonth: 23.0,
  memPerGiBMonth: 3.1,
  basis: 'Ballpark on-demand x86 rates. Replace with your blended rates in data/rate-card.json.',
};

export function parseCpu(value) {
  if (value === null || value === undefined || value === '') return null;
  const text = String(value).trim();
  if (/^\d+(\.\d+)?m$/.test(text)) return Number(text.slice(0, -1)) / 1000;
  const num = Number(text);
  return Number.isFinite(num) ? num : null;
}

const MEM_UNITS = {
  Ki: 1024 ** 1, Mi: 1024 ** 2, Gi: 1024 ** 3, Ti: 1024 ** 4,
  K: 1e3, M: 1e6, G: 1e9, T: 1e12,
  k: 1e3,
};

export function parseMemGiB(value) {
  if (value === null || value === undefined || value === '') return null;
  const text = String(value).trim();
  const match = text.match(/^(\d+(?:\.\d+)?)(Ki|Mi|Gi|Ti|K|M|G|T|k)?$/);
  if (!match) return null;
  const bytes = Number(match[1]) * (match[2] ? MEM_UNITS[match[2]] : 1);
  return bytes / (1024 ** 3);
}

function round2(value) {
  return Math.round(value * 100) / 100;
}

export function monthlyCost(cpuCores, memGiB, rateCard) {
  const cpu = cpuCores ? cpuCores * rateCard.cpuPerCoreMonth : 0;
  const mem = memGiB ? memGiB * rateCard.memPerGiBMonth : 0;
  return round2(cpu + mem);
}

function isBound(unitRow) {
  return Boolean(unitRow && unitRow.target && unitRow.target !== NIL_UUID);
}

const LEFTOVER_PATTERNS = [
  {pattern: /-live-\d{8}$/, why: 'date-stamped one-off run'},
  {pattern: /ephemeral/, why: 'named ephemeral'},
  {pattern: /-sketch|^route-sketch/, why: 'named as a sketch'},
  {pattern: /-demo\b|demo-base/, why: 'named as a demo'},
];

function priced(monthly, claim, rateCard) {
  return {
    monthly: round2(monthly),
    currency: rateCard.currency,
    claim,
    basis: rateCard.basis,
  };
}

// containers: rows from the org sweep (space, unit, kind, workload, container,
// replicas, cpu_req, cpu_lim, mem_req, mem_lim, eph_lim — quantities as strings).
// units: rows from the unit listing (space, unit, target, live, head, updated).
export function analyze({containers = [], units = []}, rateCard = defaultRateCard, now = null) {
  const findings = [];
  const unitIndex = new Map(units.map(u => [`${u.space}/${u.unit}`, u]));

  const enriched = containers.map(row => {
    const unitRow = unitIndex.get(`${row.space}/${row.unit}`) || null;
    return {
      ...row,
      replicas: Number(row.replicas) || 1,
      cpuReq: parseCpu(row.cpu_req),
      cpuLim: parseCpu(row.cpu_lim),
      memReq: parseMemGiB(row.mem_req),
      memLim: parseMemGiB(row.mem_lim),
      ephLim: parseMemGiB(row.eph_lim),
      bound: isBound(unitRow),
    };
  });

  // R1: containers with no requests. The scheduler cannot bin-pack these, so
  // nodes get sized by guesswork. Not priced as savings (no usage data), but
  // the fix is the entry ticket to every other saving.
  for (const row of enriched) {
    if (row.cpuReq !== null && row.memReq !== null) continue;
    const missing = [row.cpuReq === null ? 'cpu' : null, row.memReq === null ? 'memory' : null].filter(Boolean);
    const exposure = (row.cpuLim || row.memLim)
      ? monthlyCost((row.cpuLim || 0) * row.replicas, (row.memLim || 0) * row.replicas, rateCard)
      : null;
    findings.push({
      rule: 'MISSING_REQUESTS',
      severity: 'high',
      space: row.space,
      unit: row.unit,
      workload: `${row.workload}/${row.container}`,
      evidence: {
        missing,
        replicas: row.replicas,
        limits: {cpu: row.cpu_lim || null, memory: row.mem_lim || null},
        bound: row.bound,
      },
      priced: exposure !== null
        ? {...priced(exposure, 'exposure-at-limits-not-savings', rateCard)}
        : null,
      recommendation: {
        summary: `Set explicit ${missing.join(' and ')} requests so capacity is scheduled, priced, and governable.`,
        preview: `cub function set --space ${row.space} --unit ${row.unit} set-container-resources-defaults --dry-run -o mutations`,
        gate: 'Review the dry-run diff, then rerun without --dry-run behind an approved scope.',
      },
    });
  }

  // R2: multi-replica workloads outside prod-named spaces. Priced from
  // requests only, and only when the Unit is actually bound to a Target.
  for (const row of enriched) {
    if (row.replicas < 2) continue;
    if (/prod/.test(row.space)) continue;
    const perReplica = monthlyCost(row.cpuReq || 0, row.memReq || 0, rateCard);
    const saveable = perReplica * (row.replicas - 1);
    const hasRequests = row.cpuReq !== null || row.memReq !== null;
    findings.push({
      rule: 'NONPROD_REPLICAS',
      severity: 'medium',
      space: row.space,
      unit: row.unit,
      workload: `${row.workload}/${row.container}`,
      evidence: {replicas: row.replicas, spaceLooksNonProd: true, bound: row.bound, hasRequests},
      priced: hasRequests && saveable > 0
        ? priced(saveable, row.bound ? 'monthly-savings-if-scaled-to-1' : 'configured-cost-only-unit-not-bound', rateCard)
        : null,
      recommendation: {
        summary: `Scale ${row.workload} to 1 replica in this non-prod space, or label the space prod if that is wrong.`,
        preview: `cub function set --space ${row.space} --unit ${row.unit} set-replicas 1 --dry-run -o mutations`,
        gate: 'Approval scoped to this exact Unit and revision before any live change.',
      },
    });
  }

  // R3: spaces that look like leftovers. Their whole configured request cost
  // is reclaimable if decommissioned — claimed only for bound units.
  const spaceRows = new Map();
  for (const row of enriched) {
    if (!spaceRows.has(row.space)) spaceRows.set(row.space, []);
    spaceRows.get(row.space).push(row);
  }
  for (const [space, rows] of spaceRows) {
    const hit = LEFTOVER_PATTERNS.find(({pattern}) => pattern.test(space));
    if (!hit) continue;
    const boundRows = rows.filter(r => r.bound);
    const configured = rows.reduce(
      (sum, r) => sum + monthlyCost((r.cpuReq || 0) * r.replicas, (r.memReq || 0) * r.replicas, rateCard), 0);
    const boundConfigured = boundRows.reduce(
      (sum, r) => sum + monthlyCost((r.cpuReq || 0) * r.replicas, (r.memReq || 0) * r.replicas, rateCard), 0);
    findings.push({
      rule: 'LEFTOVER_SPACE',
      severity: boundRows.length ? 'high' : 'medium',
      space,
      unit: null,
      workload: null,
      evidence: {
        why: hit.why,
        workloadContainers: rows.length,
        boundUnits: boundRows.length,
        configuredMonthlyRequestCost: round2(configured),
      },
      priced: boundRows.length
        ? priced(boundConfigured, 'reclaimable-if-decommissioned', rateCard)
        : (configured > 0 ? priced(configured, 'configured-cost-only-no-units-bound', rateCard) : null),
      recommendation: {
        summary: `Confirm this space is a leftover (${hit.why}), then decommission it through its gates.`,
        preview: `cub unit list --space ${space} --select "TargetID,LiveRevisionNum,UpdatedAt"`,
        gate: 'Delete and Destroy Gates stay authoritative: review owners, remove gates deliberately, never bulk-delete.',
      },
    });
  }

  // R4: wide limit:request gaps. Hygiene only — limits do not drive node
  // provisioning, so this is never priced as savings.
  for (const row of enriched) {
    if (!row.cpuReq || !row.cpuLim) continue;
    const ratio = row.cpuLim / row.cpuReq;
    if (ratio <= 3) continue;
    findings.push({
      rule: 'LIMIT_HEADROOM',
      severity: 'low',
      space: row.space,
      unit: row.unit,
      workload: `${row.workload}/${row.container}`,
      evidence: {cpuRequest: row.cpu_req, cpuLimit: row.cpu_lim, ratio: round2(ratio), bound: row.bound},
      priced: null,
      recommendation: {
        summary: 'Tighten the cpu limit toward observed usage; wide headroom hides real sizing.',
        preview: `cub function set --space ${row.space} --unit ${row.unit} set-container-resources-defaults --dry-run -o mutations`,
        gate: 'Needs usage data before acting; do not change limits on estimates alone.',
      },
    });
  }

  // R5: many spaces from the same stack family. Evidence-only: humans decide
  // which copies matter, the engine just totals what each copy asks for.
  const families = new Map();
  for (const [space, rows] of spaceRows) {
    const family = space.replace(/-\d+-\d+-\d+.*$/, '').replace(/-(prod|staging|stage|dev|base|default).*$/, '');
    if (!families.has(family)) families.set(family, []);
    families.get(family).push({space, rows});
  }
  for (const [family, members] of families) {
    if (members.length < 3) continue;
    const totalConfigured = members.reduce(
      (sum, m) => sum + m.rows.reduce(
        (s, r) => s + monthlyCost((r.cpuReq || 0) * r.replicas, (r.memReq || 0) * r.replicas, rateCard), 0), 0);
    findings.push({
      rule: 'STACK_COPIES',
      severity: 'low',
      space: null,
      unit: null,
      workload: family,
      evidence: {
        spaces: members.map(m => m.space),
        copies: members.length,
        configuredMonthlyRequestCost: round2(totalConfigured),
      },
      priced: null,
      recommendation: {
        summary: `${members.length} spaces carry the ${family} stack. Confirm each copy is still needed.`,
        preview: `cub unit list --space "${family}*" --select "TargetID,UpdatedAt"`,
        gate: 'Review only. Consolidation is a human decision per copy.',
      },
    });
  }

  const severityOrder = {high: 0, medium: 1, low: 2};
  findings.sort((a, b) =>
    (severityOrder[a.severity] - severityOrder[b.severity])
    || ((b.priced?.monthly || 0) - (a.priced?.monthly || 0)));

  const boundUnits = units.filter(isBound);
  const configuredMonthly = enriched.reduce(
    (sum, r) => sum + monthlyCost((r.cpuReq || 0) * r.replicas, (r.memReq || 0) * r.replicas, rateCard), 0);
  const savings = findings
    .filter(f => f.priced && (f.priced.claim === 'monthly-savings-if-scaled-to-1' || f.priced.claim === 'reclaimable-if-decommissioned'))
    .reduce((sum, f) => sum + f.priced.monthly, 0);

  return {
    totals: {
      containersScanned: enriched.length,
      workloadUnits: new Set(enriched.map(r => `${r.space}/${r.unit}`)).size,
      spacesWithWorkloads: spaceRows.size,
      unitsInOrg: units.length,
      unitsBoundToTargets: boundUnits.length,
      containersMissingRequests: enriched.filter(r => r.cpuReq === null || r.memReq === null).length,
      configuredMonthlyRequestCost: round2(configuredMonthly),
      claimedMonthlySavings: round2(savings),
      savingsClaimRule: 'Only bound Units count toward claimed savings; everything else is configured-cost-only.',
    },
    findings,
    rateCard,
    generatedAt: now,
  };
}
