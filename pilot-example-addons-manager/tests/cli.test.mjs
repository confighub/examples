import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { spawnSync } from 'node:child_process';

const workflow = JSON.parse(await readFile('data/operational-workflow.json', 'utf8'));

function run(args, expectedStatus = 0) {
  const result = spawnSync(process.execPath, ['cli.mjs', ...args], {
    cwd: process.cwd(),
    encoding: 'utf8',
  });
  assert.equal(result.status, expectedStatus, result.stderr || result.stdout);
  return JSON.parse(result.stdout);
}

const preflight = run(['preflight', '--json']);
assert.match(preflight.status, /PASS|WATCH/);
assert.equal(preflight.sharedContract, 'data/operational-workflow.json');
assert.ok(preflight.commandLoop.some(row => row.command === 'preflight'));
assert.ok(preflight.commandLoop.some(row => row.command === 'approve/commit'));
assert.ok(preflight.commandLoop.every(row => row.harnessStep));

const mapped = run(['map', '--json']);
assert.equal(mapped.scopeModel.userFacingScope, 'Variant');
assert.deepEqual(mapped.variants.map(row => row.id), workflow.variants.map(row => row.id));

const findings = run(['findings', '--json']);
assert.equal(findings.status, 'WATCH');
assert.ok(findings.findings.some(row => row.code === 'LIVE_BINDINGS_MISSING'));

const preview = run(['preview', '--variant', workflow.variants[0].id, '--json']);
assert.equal(preview.status, 'PREVIEW_READY');
assert.equal(preview.variant.id, workflow.variants[0].id);
assert.equal(preview.mutation, 'none');

// Expected operational blocker: typed BLOCK at exit 0, not a shell failure.
const commit = run(['commit', '--variant', workflow.variants[0].id, '--json'], 0);
assert.equal(commit.verdict, 'BLOCK');
assert.equal(commit.reason, 'APPROVED_CONFIGHUB_MUTATION_REQUIRED');
assert.equal(commit.status, 'COMMIT_BLOCKED');
assert.equal(commit.error, 'APPROVED_CONFIGHUB_MUTATION_REQUIRED');
assert.match(commit.message, /approved scoped ConfigHub mutation/);

const verify = run(['verify', '--json']);
assert.equal(verify.status, 'WATCH');
assert.equal(verify.liveBindings, 'LIVE_BINDINGS_MISSING');

const receipt = run(['receipt', '--json']);
assert.equal(receipt.status, 'WAITING_FOR_LIVE_PROOF');
assert.equal(receipt.liveBindings, 'LIVE_BINDINGS_MISSING');

const guardrails = run(['guardrails', '--json']);
assert.equal(guardrails.status, 'INFO');
assert.match(guardrails.message, /common but not universal/);
