import assert from 'node:assert/strict';
import { mkdtemp, mkdir, copyFile, readFile } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { spawnSync } from 'node:child_process';

const workflow = JSON.parse(await readFile('data/operational-workflow.json', 'utf8'));

// This file asserts the FRESH-deployment contract, so it runs the CLI from a
// scratch dir holding the shared workflow contract but none of the
// deployment-local live files (live-bindings.json, cost-findings.json).
const cliPath = resolve('cli.mjs');
const scratch = await mkdtemp(join(tmpdir(), 'cli-contract-'));
await mkdir(join(scratch, 'data'));
await copyFile('data/operational-workflow.json', join(scratch, 'data', 'operational-workflow.json'));

function run(args, expectedStatus = 0) {
  const result = spawnSync(process.execPath, [cliPath, ...args], {
    cwd: scratch,
    encoding: 'utf8',
  });
  assert.equal(result.status, expectedStatus, result.stderr || result.stdout);
  return JSON.parse(result.stdout);
}

const preflight = run(['preflight', '--json']);
assert.match(preflight.status, /PASS|WATCH/);
// Result Contract: every read command carries a typed verdict and stable reason.
assert.equal(preflight.verdict, preflight.status);
assert.equal(preflight.reason, preflight.liveBindings);
assert.equal(preflight.sharedContract, 'data/operational-workflow.json');
assert.ok(preflight.commandLoop.some(row => row.command === 'preflight'));
assert.ok(preflight.commandLoop.some(row => row.command === 'approve/commit'));
assert.ok(preflight.commandLoop.every(row => row.harnessStep));

const mapped = run(['map', '--json']);
assert.equal(mapped.verdict, 'PASS');
assert.equal(mapped.reason, 'VARIANT_SCOPE_MAPPED');
assert.equal(mapped.scopeModel.userFacingScope, 'Variant');
assert.deepEqual(mapped.variants.map(row => row.id), workflow.variants.map(row => row.id));

const findings = run(['findings', '--json']);
assert.equal(findings.status, 'WATCH');
assert.equal(findings.verdict, 'WATCH');
assert.equal(findings.reason, 'LIVE_BINDINGS_MISSING');
assert.ok(findings.findings.some(row => row.code === 'LIVE_BINDINGS_MISSING'));
assert.ok(findings.findings.some(row => row.code === 'COST_SWEEP_NOT_RUN'));

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
assert.equal(verify.verdict, 'WATCH');
assert.equal(verify.reason, 'LIVE_BINDINGS_MISSING');
assert.equal(verify.liveBindings, 'LIVE_BINDINGS_MISSING');

const receipt = run(['receipt', '--json']);
assert.equal(receipt.status, 'WAITING_FOR_LIVE_PROOF');
assert.equal(receipt.verdict, 'WATCH');
assert.equal(receipt.reason, 'LIVE_BINDINGS_MISSING');
assert.equal(receipt.liveBindings, 'LIVE_BINDINGS_MISSING');

const guardrails = run(['guardrails', '--json']);
assert.equal(guardrails.status, 'INFO');
assert.match(guardrails.message, /common but not universal/);
