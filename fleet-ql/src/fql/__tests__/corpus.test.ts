// Parser robustness corpus. A broad set of realistic fleet queries across every
// ConfigHub domain (units, revisions, spaces, gates, triggers, filters,
// events, resources) that the parser MUST accept. The parser is table/column
// agnostic — it validates syntax shape, not catalog membership — so these
// exercise the grammar, not the planner. Field names here deliberately mirror
// ConfigHub's raw PascalCase wire attributes (see libra/internal/models/*.go)
// to prove the grammar accepts them (incl. keyword collisions like From/Source);
// FQL's actual column surface is camelCase (see schema.ts).

import { describe, expect, it } from 'vitest';

import { parse } from '../parser';

/** Queries that must parse without error. Grouped by the fleet question they answer. */
const VALID: Record<string, string[]> = {
  units: [
    // drift: unapplied changes (ConfigHub's documented expression — column vs column)
    'SELECT slug, space FROM units WHERE HeadRevisionNum > LiveRevisionNum',
    // never applied
    'SELECT slug FROM units WHERE LiveRevisionNum = 0',
    // clones of a specific upstream
    "SELECT slug FROM units WHERE UpstreamRevisionNum > 0 AND UpstreamUnitID = 'abc-123'",
    // gated units, and a specific gate key (bracket subscript with the / pattern)
    'SELECT slug, gates FROM units WHERE gates > 0 ORDER BY gates DESC',
    "SELECT slug FROM units u WHERE u.ApplyGates['sec-demo-policy/no-critical-cves/vet-celexpr'] = true",
    // ownership rollup
    "SELECT space, COUNT(*) AS n FROM units WHERE labels.team = 'payments' GROUP BY space ORDER BY n DESC",
    // toolchain filter with IN
    "SELECT slug FROM units WHERE toolchain IN ('Kubernetes/YAML', 'AppConfig/YAML')",
    // detached / untargeted
    'SELECT slug FROM units WHERE target IS NULL',
    'SELECT slug FROM units WHERE target IS NOT NULL AND HeadRevisionNum != LiveRevisionNum',
  ],
  revisions: [
    // audit trail, newest first
    "SELECT RevisionNum, Description, UserID FROM revisions WHERE unit = 'checkout' ORDER BY RevisionNum DESC LIMIT 10",
    // automated vs human changes
    "SELECT RevisionNum FROM revisions WHERE Source = 'Trigger'",
    "SELECT RevisionNum, Source FROM revisions WHERE Source IN ('UpdateUnit', 'PatchUnit', 'UpgradeUnit')",
    // time-bounded history
    "SELECT RevisionNum FROM revisions WHERE CreatedAt > '2026-06-01T00:00:00Z' ORDER BY CreatedAt DESC",
    // applied revisions only
    'SELECT RevisionNum FROM revisions WHERE LiveAt IS NOT NULL',
  ],
  spaces: [
    "SELECT slug FROM spaces WHERE labels.env = 'prod'",
    "SELECT slug, displayName FROM spaces WHERE slug LIKE 'team-%' AND labels.region = 'us-east'",
    "SELECT labels.env AS env, COUNT(*) AS n FROM spaces GROUP BY labels.env",
  ],
  gates: [
    // policy audit — what's blocked, and by which gate
    "SELECT unit FROM units WHERE ApplyGates['sec-demo-policy/no-critical-cves/vet-celexpr'] = true",
    "SELECT unit FROM units WHERE ApplyWarnings['sec-demo-policy/no-latest-tag/vet-celexpr'] = true",
  ],
  triggers_filters: [
    "SELECT slug FROM triggers WHERE Disabled = false AND Warn = true",
    "SELECT slug FROM triggers WHERE Event = 'Mutation' AND ToolchainType = 'Kubernetes/YAML'",
    "SELECT slug FROM filters WHERE From = 'Unit' AND ResourceType = 'apps/v1/Deployment'",
  ],
  events: [
    // did it deploy / what failed
    "SELECT unit, Result FROM events WHERE Action = 'Apply' AND Status = 'Failed'",
    "SELECT unit FROM events WHERE Result IN ('ApplyFailed', 'ApplyWaitFailed', 'DestroyFailed')",
    "SELECT unit FROM events WHERE Action = 'Refresh' AND Result = 'RefreshAndDrifted'",
  ],
  resources: [
    // the all-kinds, raw-path, bracket-key cases
    "SELECT unit, kind, metadata.name FROM resources WHERE kind = 'Service'",
    "SELECT unit FROM resources r WHERE r.kind = 'Deployment' AND `spec.template.spec.containers.*.image` ~ ':latest'",
    "SELECT unit, metadata.annotations['sec-scanner.confighub.com/max-severity'] AS sev FROM resources WHERE metadata.annotations['sec-scanner.confighub.com/max-severity'] = 'CRITICAL'",
    "SELECT unit FROM resources WHERE spec.replicas > 1 AND spec.containers[0].image NOT LIKE 'registry.internal/%'",
    "SELECT unit FROM resources WHERE `spec.template.spec.containers.*.resources.limits.cpu` IS NULL",
  ],
  complex: [
    // deeply nested boolean logic + parens
    "SELECT slug FROM units WHERE (labels.env = 'prod' OR labels.env = 'staging') AND (gates > 0 OR HeadRevisionNum > LiveRevisionNum)",
    // NOT with De Morgan shape
    "SELECT slug FROM units WHERE NOT (toolchain = 'Kubernetes/YAML' AND target IS NULL)",
    // multiple ORDER BY keys, mixed direction
    "SELECT slug, space, gates FROM units ORDER BY space ASC, gates DESC, slug LIMIT 50",
    // aggregate with ordering by the aggregate alias
    "SELECT space, MAX(HeadRevisionNum) AS maxrev FROM units GROUP BY space ORDER BY maxrev DESC",
    // comment + multiline
    "SELECT slug -- the unit\nFROM units\nWHERE space LIKE 'sec-demo-%' -- demo only\n",
  ],
};

describe('parser corpus — must parse', () => {
  for (const [domain, queries] of Object.entries(VALID)) {
    describe(domain, () => {
      for (const q of queries) {
        it(q.replace(/\s+/g, ' ').slice(0, 72), () => {
          expect(() => parse(q)).not.toThrow();
        });
      }
    });
  }
});

/** Malformed queries that MUST be rejected with a positioned error. */
const INVALID: [string, string][] = [
  ['missing FROM', 'SELECT slug units'],
  ['dangling operator', "SELECT slug FROM units WHERE slug ="],
  ['unbalanced paren', "SELECT slug FROM units WHERE (slug = 'a'"],
  ['empty IN list', 'SELECT slug FROM units WHERE slug IN ()'],
  ['IS without NULL', 'SELECT slug FROM units WHERE slug IS'],
  ['bad LIMIT', 'SELECT slug FROM units LIMIT abc'],
  ['unterminated string', "SELECT slug FROM units WHERE slug = 'oops"],
  ['unterminated backtick', 'SELECT `spec.replicas FROM resources'],
  ['unclosed bracket', "SELECT slug FROM units WHERE annotations['k' = 'v'"],
  ['trailing junk', 'SELECT slug FROM units extra junk here'],
  ['lone AND', "SELECT slug FROM units WHERE slug = 'a' AND"],
];

describe('parser corpus — must reject', () => {
  for (const [name, q] of INVALID) {
    it(name, () => {
      expect(() => parse(q)).toThrow();
    });
  }
});
