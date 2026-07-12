import assert from 'node:assert/strict';
import { access } from 'node:fs/promises';
import { createAppServer } from '../src/server.mjs';

process.env.AUTH_MODE = 'browser-oauth';
process.env.CONFIGHUB_BASE_URL = 'https://confighub.example.test';
process.env.OAUTH_CLIENT_ID = 'client_test';

const server = createAppServer();
await new Promise((resolve, reject) => {
  server.once('error', reject);
  server.listen(0, '127.0.0.1', resolve);
});
const port = server.address().port;

async function json(path, options = {}) {
  const response = await fetch(`http://localhost:${port}${path}`, options);
  const data = await response.json();
  return {response, data};
}

try {
  const page = await fetch(`http://localhost:${port}/`);
  assert.equal(page.status, 200);
  const pageText = await page.text();
  assert.match(pageText, /ConfigHub operational app/);
  assert.match(pageText, /Operational readiness/);
  assert.match(pageText, /Governed action contract/);

  const callbackPage = await fetch(`http://localhost:${port}/callback`);
  assert.equal(callbackPage.status, 200);
  assert.match(await callbackPage.text(), /ConfigHub operational app/);

  const config = await json('/app/config');
  assert.equal(config.response.status, 200);
  assert.equal(config.data.authMode, 'browser-oauth');

  const workflow = await json('/api/workflow');
  assert.equal(workflow.response.status, 200);
  assert.ok(workflow.data.variants.length >= 1);
  assert.equal(workflow.data.governedAction.kind, 'ConfigHub-governed-action.v0');
  assert.ok(workflow.data.governedAction.operation);

  const variants = await json('/api/variants');
  assert.equal(variants.response.status, 200);
  assert.ok(variants.data.variants.every(row => row.variant));

  const bindings = await json('/api/bindings');
  assert.equal(bindings.response.status, 200);
  // Deployment-local: the bindings file may legitimately exist on a bound
  // deployment. Assert the reported status is consistent with the filesystem.
  const bindingsFileExists = await access('data/live-bindings.json').then(() => true, () => false);
  assert.equal(bindings.data.status === 'LIVE_BINDINGS_MISSING', !bindingsFileExists);

  const missingAuth = await json('/api/preview', {
    method: 'POST',
    headers: {'content-type': 'application/json'},
    body: JSON.stringify({variantId: variants.data.variants[0].id}),
  });
  if (true) {
    assert.equal(missingAuth.response.status, 401);
    assert.equal(missingAuth.data.error, 'SIGN_IN_REQUIRED');
  } else {
    assert.equal(missingAuth.response.status, 200);
  }

  const preview = await json('/api/preview', {
    method: 'POST',
    headers: {'content-type': 'application/json', authorization: 'Bearer test-token'},
    body: JSON.stringify({variantId: variants.data.variants[0].id}),
  });
  assert.equal(preview.response.status, 200);
  assert.equal(preview.data.status, 'PREVIEW_READY');

  const apply = await json('/api/apply', {
    method: 'POST',
    headers: {'content-type': 'application/json', authorization: 'Bearer test-token'},
    body: JSON.stringify({variantId: variants.data.variants[0].id}),
  });
  assert.equal(apply.response.status, 409);
  // Apply must always be refused with a typed reason: missing bindings on a
  // fresh deployment, missing executor on a bound one. Never a success.
  assert.ok(['LIVE_BINDINGS_REQUIRED', 'LIVE_ACTION_EXECUTOR_REQUIRED'].includes(apply.data.error), apply.data.error);
} finally {
  await new Promise(resolve => server.close(resolve));
}
