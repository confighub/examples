import { spawnSync } from 'node:child_process';
import { readFile, writeFile } from 'node:fs/promises';

// Fleet policy: one browser client per app per org. The client id is a
// public identifier, but the registration itself is fleet inventory: a
// successful run records the client id, redirect URIs, and timestamps in
// confighub/registry/fleet-record.json. Redirect-URI changes append a
// recorded mutation; nothing edits the registration silently.
const FLEET_RECORD_PATH = 'confighub/registry/fleet-record.json';
const name = process.env.OAUTH_CLIENT_NAME || 'cost-management-app-local';
const redirectUri = process.env.OAUTH_REDIRECT_URI || 'http://localhost:5173/';

async function loadRecord() {
  try {
    return JSON.parse(await readFile(FLEET_RECORD_PATH, 'utf8'));
  } catch {
    return null;
  }
}

const record = await loadRecord();
const recordedClient = record && record.oauthClient ? record.oauthClient : null;
if (recordedClient && recordedClient.state === 'revoked') {
  // Registering over a revoked record would skip the decommission decision:
  // typed BLOCK at exit 0, no client is created, nothing is mutated.
  console.log(JSON.stringify({
    verdict: 'BLOCK',
    reason: 'CLIENT_REVOKED',
    status: 'OAUTH_REGISTER_BLOCKED_REVOKED',
    message: 'The fleet record marks this client registration revoked at decommission. Run node lifecycle.mjs rollback --json to restore the record before registering again.',
  }, null, 2));
  process.exit(0);
}

function runCub(args) {
  return spawnSync('cub', args, {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  });
}

function parseJson(result, action) {
  if (result.status !== 0) {
    throw new Error(`${action} failed: ${result.stderr || result.stdout}`);
  }
  return JSON.parse(result.stdout);
}

let result = runCub(['oauthclient', 'create', name, '--redirect-uri', redirectUri, '-o', 'json']);
let action = 'created';
if (result.status !== 0) {
  const message = `${result.stderr}\n${result.stdout}`.toLowerCase();
  if (!message.includes('already') && !message.includes('exists') && !message.includes('duplicate')) {
    throw new Error(`cub oauthclient create failed: ${result.stderr || result.stdout}`);
  }
  result = runCub(['oauthclient', 'get', name, '-o', 'json']);
  action = 'existing';
}

const client = parseJson(result, `cub oauthclient ${action === 'created' ? 'create' : 'get'}`);
const clientId = client.ClientID || client.client_id || client.clientId || '';
if (!clientId) {
  throw new Error('OAuth client response did not include ClientID');
}

let fleetRecord = 'record-missing';
if (record) {
  const now = new Date().toISOString();
  const target = record.oauthClient || (record.oauthClient = {
    state: 'unregistered',
    stateMachine: ['unregistered', 'registered', 'rotated', 'revoked'],
    clientId: '',
    clientName: name,
    org: '',
    redirectUris: [],
    requiredRedirectUris: [],
    createdAt: '',
    lastRotatedAt: '',
    mutations: [],
  });
  const redirectUris = Array.isArray(target.redirectUris) ? target.redirectUris : [];
  const redirectUriAdded = !redirectUris.includes(redirectUri);
  if (redirectUriAdded) redirectUris.push(redirectUri);
  target.redirectUris = redirectUris;
  target.state = 'registered';
  target.clientId = clientId;
  target.clientName = name;
  if (!target.createdAt) target.createdAt = now;
  target.mutations = [...(Array.isArray(target.mutations) ? target.mutations : []), {
    kind: 'register',
    action,
    clientId,
    redirectUri,
    redirectUriAdded,
    at: now,
  }];
  record.lastLifecycleEvent = {command: 'oauth-register', at: now};
  await writeFile(FLEET_RECORD_PATH, JSON.stringify(record, null, 2) + '\n', 'utf8');
  fleetRecord = 'recorded';
}

console.log(JSON.stringify({
  status: 'OAUTH_CLIENT_READY',
  action,
  name,
  redirectUri,
  clientId,
  fleetRecord,
  clientState: 'registered',
  env: {
    CONFIGHUB_BASE_URL: process.env.CONFIGHUB_BASE_URL || 'https://hub.confighub.com',
    OAUTH_CLIENT_ID: clientId,
    VITE_CONFIGHUB_BASE_URL: process.env.CONFIGHUB_BASE_URL || 'https://hub.confighub.com',
    VITE_OAUTH_CLIENT_ID: clientId,
    PORT: process.env.PORT || '5173',
  },
  run: `VITE_CONFIGHUB_BASE_URL=${process.env.CONFIGHUB_BASE_URL || 'https://hub.confighub.com'} VITE_OAUTH_CLIENT_ID=${clientId} npm run ui:dev`,
}, null, 2));
