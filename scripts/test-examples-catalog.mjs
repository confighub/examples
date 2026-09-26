import assert from 'node:assert/strict';
import { existsSync, readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { eligible, findExamples, getExample } from './examples-catalog.mjs';

const root = resolve(fileURLToPath(new URL('..', import.meta.url)));
const catalog = JSON.parse(readFileSync(resolve(root, 'catalog/examples.json'), 'utf8'));
const qualification = JSON.parse(readFileSync(resolve(root, 'catalog/qualification.json'), 'utf8'));
assert.equal(catalog.schema_version, 1);
assert.equal(qualification.schema_version, 1);
assert.ok(catalog.entries.length >= 4);
assert.ok(!/\/Users\/|\/private\/tmp\/|\/tmp\//.test(JSON.stringify(qualification)), 'qualification must not include machine paths');

const ids = new Set();
for (const entry of catalog.entries) {
  assert.match(entry.id, /^[a-z0-9]+(?:-[a-z0-9]+)*$/);
  assert.ok(!ids.has(entry.id), `duplicate ID ${entry.id}`);
  ids.add(entry.id);
  assert.equal(entry.visibility, 'public');
  assert.equal(entry.maintenance_owner, entry.source.repository);
  assert.equal(entry.maintainer_acceptance, 'pending-maintainer-review');
  assert.deepEqual(Object.keys(entry.requirements).sort(), ['connected', 'local']);
  for (const scope of ['local', 'connected']) {
    for (const field of ['credentials', 'cost', 'cleanup']) {
      assert.ok(typeof entry.requirements[scope][field] === 'string' && entry.requirements[scope][field].trim(), `${entry.id} missing ${scope}.${field}`);
    }
  }
  assert.ok(['not-tested', 'not-applicable', 'not-in-index', 'scoped-config-hub-verified'].includes(entry.requirements.connected.qualification));
  const versions = entry.requirements.local.tested_tool_versions;
  assert.ok(versions && Object.keys(versions).length, `${entry.id} missing tested tool versions`);
  for (const [tool, version] of Object.entries(versions)) {
    assert.match(tool, /^[a-z][a-z0-9-]*$/);
    assert.ok(typeof version === 'string' && (version === 'unknown' || /^[v]?[0-9]+(?:\.[0-9]+)+$/.test(version)), `${entry.id} has an unsupported version value for ${tool}`);
  }
  assert.ok(['maintained', 'candidate', 'needs-refresh', 'superseded', 'historical'].includes(entry.lifecycle));
  assert.ok(['verified-source', 'source-reviewed'].includes(entry.admission));
  assert.match(entry.source.repository, /^confighub\/[a-z0-9-]+$/);
  assert.match(entry.source.revision, /^[a-f0-9]{40}$/);
  assert.match(entry.source.path, /^[a-z0-9][a-z0-9/-]*$/);
  assert.equal(entry.source.url, `https://github.com/${entry.source.repository}/tree/${entry.source.revision}/${entry.source.path}`);
  assert.ok(entry.task && entry.effects && entry.stop_when && entry.preview.command && entry.preview.artifact);
  assert.match(entry.evidence.checked_on, /^\d{4}-\d{2}-\d{2}$/);
  for (const stage of ['static', 'connected', 'controller', 'runtime']) {
    assert.ok(entry.evidence[stage], `${entry.id} missing ${stage} evidence`);
  }
  const guidePrefix = `https://github.com/${entry.source.repository}/blob/${entry.source.revision}/`;
  for (const guide of Object.values(entry.guides)) {
    assert.ok(guide.startsWith(guidePrefix), `${entry.id} guide must be a pinned public URL`);
  }
  if (entry.source.repository === 'confighub/examples') {
    assert.ok(existsSync(resolve(root, entry.source.path)), `${entry.id} source path missing`);
    for (const guide of Object.values(entry.guides)) {
      const localPath = guide.slice(guidePrefix.length);
      assert.ok(!localPath.includes('..') && existsSync(resolve(root, localPath)), `${entry.id} guide missing: ${guide}`);
    }
  }
  if (entry.tutorial) {
    assert.ok(!entry.tutorial.includes('..') && existsSync(resolve(root, entry.tutorial)));
  }
  if (entry.admission === 'verified-source') {
    assert.equal(entry.evidence.receipt, `catalog/qualification.json#/checks/${entry.id}`);
    const check = qualification.checks[entry.id];
    assert.ok(check, `${entry.id} missing qualification receipt`);
    assert.equal(check.source_revision, entry.source.revision);
    assert.equal(check.working_directory, entry.preview.working_directory);
    assert.deepEqual(check.tested_tool_versions, entry.requirements.local.tested_tool_versions);
    assert.ok(check.commands.some(record => record.command === entry.preview.command && record.exit_code === 0), `${entry.id} preview lacks a passing receipt`);
    if (entry.practice) {
      assert.ok(check.commands.some(record => record.command === entry.practice.command && record.exit_code === 0), `${entry.id} practice lacks a passing receipt`);
    }
  }
}

const firstAppDiff = qualification.checks['first-app-realistic'].commands.find(record => record.command === 'bash catalog/first-app-local-change.sh').output_projection;
const firstApp = getExample('first-app-realistic');
assert.equal(firstApp.evidence.connected_receipt, 'catalog/first-app-connected-receipt.json');
assert.equal(firstApp.requirements.connected.qualification, 'scoped-config-hub-verified');
const connectedRaw = readFileSync(resolve(root, firstApp.evidence.connected_receipt), 'utf8');
assert.ok(!/\/Users\/|\/private\/tmp\/|localhost|@/.test(connectedRaw), 'connected receipt must not include machine paths or account data');
const connected = JSON.parse(connectedRaw);
assert.equal(connected.schema_version, 1);
assert.equal(connected.example, firstApp.id);
assert.equal(connected.scope, 'confighub-only-setup-and-scoped-change');
assert.deepEqual(connected.source, {
  repository: firstApp.source.repository,
  revision: firstApp.source.revision,
  path: firstApp.source.path
});
assert.match(connected.prefix, /^teaching-app-[0-9]+$/);
assert.equal(connected.environment.server_version, 'v0.5.1');
assert.equal(connected.environment.server_cli_version, 'v0.5.1');
assert.equal(connected.environment.local_diff_cli_version, 'v0.5.1');
assert.equal(firstApp.requirements.connected.tested_tool_versions.cub, connected.environment.server_cli_version);
assert.equal(firstApp.requirements.connected.tested_tool_versions.server, connected.environment.server_version);
assert.equal(firstApp.requirements.connected.tested_tool_versions['local-diff-cub'], connected.environment.local_diff_cli_version);
assert.equal(firstApp.requirements.connected.tested_tool_versions['cub-config-plugin'], connected.environment.workshop_plugin_version);
assert.match(connected.preflight.latest_client_read, /v0\.6\.2.*v0\.5\.1.*invalid include field ComponentID/);
assert.equal(connected.setup.exit_code, 0);
assert.deepEqual([connected.setup.spaces_created, connected.setup.units_created, connected.setup.component_units, connected.setup.recipe_units, connected.setup.namespace_configuration_units], [5, 17, 15, 1, 1]);
assert.equal(connected.setup.target_argument, null);
for (const result of [connected.verification_before, connected.verification_after]) {
  assert.equal(result.ok, true);
  assert.equal(result.prefix, connected.prefix);
  assert.equal(result.targetExpected, false);
  assert.equal(result.spacesChecked.length, 5);
  assert.ok(result.spacesChecked.every(space => space.startsWith(`${connected.prefix}-`)));
}
assert.equal(connected.unit_snapshot.length, 17);
assert.ok(connected.unit_snapshot.every(unit => unit.space.startsWith(`${connected.prefix}-`)));
assert.equal(connected.change.exit_code, 0);
assert.deepEqual(connected.change.diff.summary, { added: 0, removed: 0, changed: 1, unchanged: 2 });
assert.deepEqual(connected.change.diff.changes[0].fields, [{ path: '/spec/replicas', operation: 'replace', before: 2, after: 3 }]);
assert.equal(connected.change.diff.changes[0].object.name, 'frontend');
assert.equal(connected.change.diff.changes[0].object.namespace, 'cluster-a');
assert.equal(connected.unit_snapshot.find(unit => unit.unit === 'frontend-cluster-a').data_hash, connected.change.diff.after.sha256.slice('sha256:'.length));
assert.equal(connected.cleanup.performed, false);
assert.match(connected.gui_inspection.mode, /^read-only/);
assert.deepEqual(connected.gui_inspection.route, ['Units', `${connected.prefix}-deploy-cluster-a`, 'frontend-cluster-a']);
assert.deepEqual(connected.gui_inspection.deploy_space_rows, ['cluster-a-namespace', 'backend-cluster-a', 'frontend-cluster-a', 'postgres-cluster-a']);
assert.equal(connected.gui_inspection.frontend_overview.upstream, 'frontend-recipe-us-staging');
assert.deepEqual([connected.gui_inspection.frontend_overview.revisions, connected.gui_inspection.frontend_overview.links, connected.gui_inspection.frontend_overview.target_binding], [6, 1, 'empty']);
assert.equal(connected.gui_inspection.frontend_overview.last_change, 'Functions: set-replicas');
assert.deepEqual(connected.gui_inspection.frontend_config, {
  format: 'Kubernetes/YAML', kind: 'Deployment', namespace: 'cluster-a', replicas: 3,
  image: 'ghcr.io/confighub/cubbychat/frontend:1.1.7'
});
assert.ok(!connected.not_proven.includes('GUI walkthrough'));
for (const limit of ['human tutorial acceptance', 'target binding', 'OCI or controller delivery', 'workload availability', 'rollback safety or successful rollback']) {
  assert.ok(connected.not_proven.includes(limit));
}
const frontend = readFileSync(resolve(root, 'global-app-layer/baseconfig/frontend.yaml'));
const changed = Buffer.from(frontend.toString().replace(/^  replicas: 1$/m, '  replicas: 2'));
const sha = data => 'sha256:' + createHash('sha256').update(data).digest('hex');
assert.equal(firstAppDiff.before.sha256, sha(frontend));
assert.equal(firstAppDiff.after.sha256, sha(changed));
assert.deepEqual(firstAppDiff.summary, { added: 0, removed: 0, changed: 1, unchanged: 2 });
assert.deepEqual(firstAppDiff.changes[0].fields, [{ path: '/spec/replicas', operation: 'replace', before: 1, after: 2 }]);

assert.deepEqual(findExamples().map(entry => entry.id), [
  'app-only-provider-config', 'argo-beginner-applicationset', 'config-repo-scout', 'first-app-realistic', 'flux-beginner', 'governed-helm-change', 'gpu-layered-recipe', 'layered-platform-recipe'
]);
assert.equal(findExamples({ query: 'what an app looks like' })[0].id, 'first-app-realistic');
assert.equal(findExamples({ query: 'existing Argo repository' })[0].id, 'argo-beginner-applicationset');
assert.equal(findExamples({ query: 'Flux onboarding' })[0].id, 'flux-beginner');
assert.equal(findExamples({ query: 'local diagnosis' })[0].id, 'config-repo-scout');
assert.ok(!findExamples({ query: 'promotion' }).some(entry => entry.id === 'promotion-demo-data'), 'source-reviewed promotion data must not appear as runnable');
assert.equal(findExamples({ query: 'governed change' })[0].id, 'governed-helm-change');
assert.equal(findExamples({ query: 'promotion', includeAll: true })[0].id, 'promotion-demo-data');
assert.equal(findExamples({ query: 'saved bundle', includeAll: true })[0].id, 'scout-import-from-bundle');
assert.equal(findExamples({ query: 'helm values', includeAll: true })[0].id, 'governed-helm-change');
assert.equal(findExamples({ query: 'app settings' })[0].id, 'app-only-provider-config');
assert.equal(findExamples({ query: 'platform' })[0].id, 'layered-platform-recipe');
assert.equal(findExamples({ query: 'gpu' })[0].id, 'gpu-layered-recipe');
assert.ok(catalog.entries.every(entry => getExample(entry.id)?.source.url === entry.source.url));
assert.ok(findExamples().every(eligible));

const output = JSON.parse(execFileSync(process.execPath, [resolve(root, 'scripts/examples-catalog.mjs'), 'search', 'argo', '--json'], { encoding: 'utf8' }));
assert.equal(output.schema_version, 1);
assert.equal(output.entries[0].id, 'argo-beginner-applicationset');
console.log('Example catalog schema, source links, eligibility and search passed.');
