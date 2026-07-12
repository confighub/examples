import assert from 'node:assert/strict';

const base = (process.env.CONFIGHUB_BASE_URL || process.env.CONFIGHUB_BASE || '').replace(/\/$/, '');
const clientId = process.env.OAUTH_CLIENT_ID || '';
assert.ok(base, 'CONFIGHUB_BASE_URL or CONFIGHUB_BASE is required');
assert.ok(clientId, 'OAUTH_CLIENT_ID is required');

const infoResponse = await fetch(`${base}/api/info`);
assert.equal(infoResponse.ok, true, `ConfigHub /api/info returned ${infoResponse.status}`);
const info = await infoResponse.json();
const authIssuer = info.AuthIssuer || info.auth_issuer || info.authIssuer;
let authorizationEndpoint = info.AuthorizationEndpoint || info.authorization_endpoint || info.oauth_authorization_endpoint;
let tokenEndpoint = info.TokenEndpoint || info.token_endpoint;
if ((!authorizationEndpoint || !tokenEndpoint) && authIssuer) {
  const oidcResponse = await fetch(`${authIssuer.replace(/\/$/, '')}/.well-known/openid-configuration`);
  assert.equal(oidcResponse.ok, true, `OIDC discovery returned ${oidcResponse.status}`);
  const oidc = await oidcResponse.json();
  authorizationEndpoint = authorizationEndpoint || oidc.authorization_endpoint;
  tokenEndpoint = tokenEndpoint || oidc.token_endpoint;
}
const tokenExchangeEndpoint = info.TokenExchangeEndpoint || info.token_exchange_endpoint || `${base}/auth/exchange`;
assert.ok(authIssuer || authorizationEndpoint, 'AuthIssuer or authorization endpoint missing from /api/info');
assert.ok(authorizationEndpoint, 'authorization endpoint missing from ConfigHub or OIDC discovery');
assert.ok(tokenEndpoint, 'OIDC token endpoint missing from ConfigHub or OIDC discovery');
assert.ok(tokenExchangeEndpoint, 'token exchange endpoint missing from /api/info');

const result = {
  status: 'OAUTH_DISCOVERY_READY',
  authIssuerPresent: Boolean(authIssuer),
  authorizationEndpointPresent: true,
  tokenEndpointPresent: true,
  tokenExchangeEndpointPresent: true,
  clientIdPresent: Boolean(clientId),
};

const accessToken = process.env.CONFIGHUB_ACCESS_TOKEN || '';
if (accessToken) {
  const meResponse = await fetch(`${base}/api/me`, {
    headers: {authorization: `Bearer ${accessToken}`},
  });
  assert.equal(meResponse.ok, true, `ConfigHub /api/me returned ${meResponse.status}`);
  result.status = 'OAUTH_API_ME_READY';
  result.apiMe = await meResponse.json();
}

console.log(JSON.stringify(result, null, 2));
