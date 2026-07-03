import { spawnSync } from 'node:child_process';

const name = process.env.OAUTH_CLIENT_NAME || 'add-on-manager-local';
const redirectUri = process.env.OAUTH_REDIRECT_URI || 'http://localhost:5173/callback';

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

console.log(JSON.stringify({
  status: 'OAUTH_CLIENT_READY',
  action,
  name,
  redirectUri,
  clientId,
  env: {
    CONFIGHUB_BASE_URL: process.env.CONFIGHUB_BASE_URL || 'https://hub.confighub.com',
    OAUTH_CLIENT_ID: clientId,
    PORT: process.env.PORT || '5173',
  },
  run: `CONFIGHUB_BASE_URL=${process.env.CONFIGHUB_BASE_URL || 'https://hub.confighub.com'} OAUTH_CLIENT_ID=${clientId} PORT=${process.env.PORT || '5173'} npm start`,
}, null, 2));
