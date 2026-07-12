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
const requestedOrg = process.env.CONFIGHUB_ORG || '';
const confirmed = process.argv.includes('--confirm');

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
  const result = spawnSync('cub', args, {
    encoding: 'utf8',
    stdio: ['ignore', 'pipe', 'pipe'],
  });
  if (result.error) {
    console.error(JSON.stringify({
      verdict: 'ERROR',
      reason: 'CUB_COMMAND_UNAVAILABLE',
      status: 'OAUTH_REGISTER_ERROR',
      message: String(result.error.message || result.error),
    }, null, 2));
    process.exit(1);
  }
  return result;
}

function parseJson(result, action) {
  if (result.status !== 0) {
    throw new Error(`${action} failed: ${result.stderr || result.stdout}`);
  }
  return JSON.parse(result.stdout);
}

function block(reason, message, extra = {}) {
  console.log(JSON.stringify({verdict: 'BLOCK', reason, status: 'OAUTH_REGISTER_BLOCKED', message, ...extra}, null, 2));
  process.exit(0);
}

if (!requestedOrg) {
  block('CONFIGHUB_ORG_REQUIRED', 'Set CONFIGHUB_ORG to the ConfigHub organization slug or id before registering this app.');
}
const auth = runCub(['auth', 'status']);
if (auth.status !== 0) {
  block('AUTH_REQUIRED', 'The active Cub session is not authenticated. Run cub auth login, then retry.', {detail: String(auth.stderr || auth.stdout).slice(0, 300)});
}
const contextResult = runCub(['context', 'get', '-o', 'json']);
if (contextResult.status !== 0) {
  block('CONTEXT_READ_FAILED', 'The app could not read the active Cub context.', {detail: String(contextResult.stderr || contextResult.stdout).slice(0, 300)});
}
const orgResult = runCub(['organization', 'get', requestedOrg, '-o', 'json']);
if (orgResult.status !== 0) {
  block('ORG_READ_FAILED', 'The app could not resolve CONFIGHUB_ORG through the authenticated organization API.', {detail: String(orgResult.stderr || orgResult.stdout).slice(0, 300)});
}
const context = parseJson(contextResult, 'cub context get');
const orgPayload = parseJson(orgResult, 'cub organization get');
const org = orgPayload.Organization || orgPayload.organization || orgPayload;
const contextOrgRef = String(context.coordinate?.organizationID || '');
const contextOrgName = String(context.metadata?.organizationName || '');
const organizationId = String(org.OrganizationID || org.ID || '');
const externalOrganizationId = String(org.ExternalID || '');
const orgSlug = String(org.Slug || '');
if (!contextOrgRef || !organizationId || !externalOrganizationId) {
  block('ORG_IDENTITY_INCOMPLETE', 'The active context or organization response lacks the ids needed to prove browser org scope.');
}
const contextMatches = [organizationId, externalOrganizationId].includes(contextOrgRef)
  || [orgSlug, String(org.DisplayName || '')].filter(Boolean).includes(contextOrgName);
if (!contextMatches) {
  block('ORG_MISMATCH', 'The active Cub context is not scoped to CONFIGHUB_ORG.', {
    expectedOrganizationId: organizationId,
    expectedExternalOrganizationId: externalOrganizationId,
    actualContextOrganization: contextOrgRef,
  });
}
if (!confirmed) {
  console.log(JSON.stringify({
    verdict: 'ASK',
    reason: 'CONFIRM_REQUIRED',
    status: 'OAUTH_REGISTRATION_AWAITING_CONFIRMATION',
    message: 'Review the exact organization, client name, and redirect URI, then rerun with --confirm.',
    org: String(orgSlug || org.DisplayName || requestedOrg),
    organizationId,
    externalOrganizationId,
    name,
    redirectUri,
    nextCommand: `CONFIGHUB_ORG=${requestedOrg} npm run oauth:register -- --confirm`,
  }, null, 2));
  process.exit(0);
}

let result = runCub(['oauthclient', 'create', name, '--redirect-uri', redirectUri, '-o', 'json']);
let action = 'created';
if (result.status !== 0) {
  const message = `${result.stderr}\n${result.stdout}`.toLowerCase();
  if (!message.includes('already') && !message.includes('exists') && !message.includes('duplicate')) {
    throw new Error(`cub oauthclient create failed: ${result.stderr || result.stdout}`);
  }
  action = 'existing';
}

// The create response is not proof that the provider persisted the requested
// redirect URI. Read the registered object back for both create and reuse.
result = runCub(['oauthclient', 'get', name, '-o', 'json']);
const client = parseJson(result, 'cub oauthclient get after registration');
const clientEntity = client.OAuthClient || client.oauthClient || client;
const clientId = clientEntity.ClientID || clientEntity.client_id || clientEntity.clientId || '';
if (!clientId) {
  throw new Error('OAuth client response did not include ClientID');
}
const returnedRedirectUris = clientEntity.RedirectURIs
  || clientEntity.RedirectUris
  || clientEntity.redirect_uris
  || clientEntity.redirectUris
  || [];
if (!Array.isArray(returnedRedirectUris) || !returnedRedirectUris.includes(redirectUri)) {
  block('REDIRECT_URI_NOT_REGISTERED', 'The registered OAuth client does not include the requested redirect URI. Update or rotate the registration before using it.', {
    requestedRedirectUri: redirectUri,
    registeredRedirectUris: Array.isArray(returnedRedirectUris) ? returnedRedirectUris : [],
  });
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
    organizationId: '',
    externalOrganizationId: '',
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
  target.org = String(orgSlug || org.DisplayName || requestedOrg);
  target.organizationId = organizationId;
  target.externalOrganizationId = externalOrganizationId;
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
  org: String(orgSlug || org.DisplayName || requestedOrg),
  organizationId,
  externalOrganizationId,
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
