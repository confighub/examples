const PKCE_KEY = "addon_manager_pkce";
const redirectUri = `${window.location.origin}${window.location.pathname}`;

function trimSlash(value) {
  return String(value || "").replace(/\/+$/, "");
}

function b64url(buffer) {
  return btoa(String.fromCharCode(...new Uint8Array(buffer)))
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/, "");
}

function randomString(bytes = 64) {
  return b64url(crypto.getRandomValues(new Uint8Array(bytes)).buffer);
}

async function sha256(value) {
  return b64url(await crypto.subtle.digest("SHA-256", new TextEncoder().encode(value)));
}

function decodeJwtClaims(token) {
  const payload = token.split(".")[1].replace(/-/g, "+").replace(/_/g, "/");
  return JSON.parse(atob(payload));
}

export async function discoverConfigHub(baseUrl) {
  const response = await fetch(`${trimSlash(baseUrl)}/api/info`);
  if (!response.ok) throw new Error(`/api/info failed: ${response.status}`);
  return response.json();
}

async function discoverIssuer(issuer) {
  const response = await fetch(`${trimSlash(issuer)}/.well-known/openid-configuration`);
  if (!response.ok) throw new Error(`OIDC discovery failed: ${response.status}`);
  return response.json();
}

export async function startLogin({baseUrl, clientId}) {
  const info = await discoverConfigHub(baseUrl);
  if (!info.AuthIssuer || !info.TokenExchangeEndpoint) {
    throw new Error("This ConfigHub server is not configured for browser token exchange.");
  }
  const oidc = await discoverIssuer(info.AuthIssuer);
  const verifier = randomString();
  const state = randomString(16);
  const challenge = await sha256(verifier);
  sessionStorage.setItem(PKCE_KEY, JSON.stringify({
    verifier,
    state,
    clientId,
    baseUrl,
    tokenEndpoint: oidc.token_endpoint,
    exchangeEndpoint: info.TokenExchangeEndpoint,
  }));

  const authUrl = new URL(oidc.authorization_endpoint);
  authUrl.search = new URLSearchParams({
    response_type: "code",
    client_id: clientId,
    redirect_uri: redirectUri,
    scope: "openid email profile organization",
    code_challenge: challenge,
    code_challenge_method: "S256",
    state,
  }).toString();
  window.location.assign(authUrl.toString());
}

let pendingCompletion = null;

export function completeLoginFromRedirect() {
  if (!pendingCompletion) pendingCompletion = completeLogin();
  return pendingCompletion;
}

async function completeLogin() {
  const params = new URLSearchParams(window.location.search);
  const error = params.get("error");
  if (error) {
    history.replaceState({}, "", redirectUri);
    throw new Error(`Identity provider returned ${error}: ${params.get("error_description") || ""}`);
  }
  const code = params.get("code");
  if (!code) return null;

  const savedRaw = sessionStorage.getItem(PKCE_KEY);
  sessionStorage.removeItem(PKCE_KEY);
  history.replaceState({}, "", redirectUri);
  if (!savedRaw) throw new Error("No saved login state. Start sign-in again.");
  const saved = JSON.parse(savedRaw);
  if (params.get("state") !== saved.state) throw new Error("Login state mismatch.");

  const tokenResponse = await fetch(saved.tokenEndpoint, {
    method: "POST",
    headers: {"content-type": "application/x-www-form-urlencoded"},
    body: new URLSearchParams({
      grant_type: "authorization_code",
      code,
      redirect_uri: redirectUri,
      client_id: saved.clientId,
      code_verifier: saved.verifier,
    }),
  });
  if (!tokenResponse.ok) {
    throw new Error(`Identity token exchange failed: ${tokenResponse.status} ${await tokenResponse.text()}`);
  }
  const identityToken = await tokenResponse.json();

  const exchangeResponse = await fetch(saved.exchangeEndpoint, {
    method: "POST",
    headers: {"content-type": "application/x-www-form-urlencoded"},
    body: new URLSearchParams({
      grant_type: "urn:ietf:params:oauth:grant-type:token-exchange",
      subject_token: identityToken.access_token,
      subject_token_type: "urn:ietf:params:oauth:token-type:access_token",
    }),
  });
  if (!exchangeResponse.ok) {
    throw new Error(`ConfigHub token exchange failed: ${exchangeResponse.status} ${await exchangeResponse.text()}`);
  }
  const minted = await exchangeResponse.json();

  return {
    baseUrl: saved.baseUrl,
    accessToken: minted.access_token,
    organizationId: minted.organization_id,
    identityClaims: decodeJwtClaims(identityToken.access_token),
  };
}

export async function callConfigHub(session, path, {asText = false} = {}) {
  const response = await fetch(`${trimSlash(session.baseUrl)}${path}`, {
    headers: {authorization: `Bearer ${session.accessToken}`},
  });
  const body = asText ? await response.text() : await response.json().catch(() => ({}));
  if (!response.ok) {
    const detail = typeof body === "string" ? body : JSON.stringify(body);
    throw new Error(`${path} failed: ${response.status} ${detail}`);
  }
  return body;
}
