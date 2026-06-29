import assert from "node:assert/strict";

const base = (process.env.CONFIGHUB_BASE || process.env.CONFIGHUB_BASE_URL || "").replace(/\/+$/, "");
const clientId = process.env.OAUTH_CLIENT_ID || "";

assert.ok(base, "CONFIGHUB_BASE or CONFIGHUB_BASE_URL is required");
assert.ok(clientId, "OAUTH_CLIENT_ID is required");

const infoResponse = await fetch(`${base}/api/info`);
assert.equal(infoResponse.ok, true, `ConfigHub /api/info returned ${infoResponse.status}`);
const info = await infoResponse.json();

assert.ok(info.AuthIssuer || info.auth_issuer, "AuthIssuer missing from /api/info");
assert.ok(info.TokenExchangeEndpoint || info.token_exchange_endpoint, "TokenExchangeEndpoint missing from /api/info");

const result = {
  status: "OAUTH_DISCOVERY_READY",
  authIssuerPresent: true,
  tokenExchangeEndpointPresent: true,
  clientIdPresent: Boolean(clientId),
};

const accessToken = process.env.CONFIGHUB_ACCESS_TOKEN || "";
if (accessToken) {
  const meResponse = await fetch(`${base}/api/me`, {
    headers: {authorization: `Bearer ${accessToken}`},
  });
  assert.equal(meResponse.ok, true, `ConfigHub /api/me returned ${meResponse.status}`);
  result.status = "OAUTH_API_ME_READY";
  result.apiMe = await meResponse.json();
}

console.log(JSON.stringify(result, null, 2));
