const ACCESS_TOKEN_KEY = 'confighub_app_access_token';
const VERIFIER_KEY = 'confighub_app_code_verifier';
const STATE_KEY = 'confighub_app_oauth_state';

function base64Url(bytes) {
  const binary = String.fromCharCode(...bytes);
  return btoa(binary).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/g, '');
}

function randomString(size = 32) {
  const bytes = new Uint8Array(size);
  crypto.getRandomValues(bytes);
  return base64Url(bytes);
}

async function sha256(value) {
  const bytes = new TextEncoder().encode(value);
  const digest = await crypto.subtle.digest('SHA-256', bytes);
  return base64Url(new Uint8Array(digest));
}

function callbackUrl(config) {
  return `${location.origin}${config.callbackPath || '/callback'}`;
}

async function discover(config) {
  const base = (config.configHubBaseUrl || '').replace(/\/$/, '');
  if (!base) {
    throw new Error('CONFIGHUB_BASE_URL is required for browser OAuth');
  }
  const response = await fetch(`${base}/api/info`);
  if (!response.ok) {
    throw new Error('ConfigHub discovery failed');
  }
  const info = await response.json();
  const authIssuer = info.AuthIssuer || info.auth_issuer || info.authIssuer;
  let authorizationEndpoint = info.AuthorizationEndpoint || info.authorization_endpoint || info.oauth_authorization_endpoint;
  let tokenEndpoint = info.TokenEndpoint || info.token_endpoint;
  if ((!authorizationEndpoint || !tokenEndpoint) && authIssuer) {
    const oidcResponse = await fetch(`${authIssuer.replace(/\/$/, '')}/.well-known/openid-configuration`);
    if (!oidcResponse.ok) {
      throw new Error('OpenID discovery failed');
    }
    const oidc = await oidcResponse.json();
    authorizationEndpoint = authorizationEndpoint || oidc.authorization_endpoint;
    tokenEndpoint = tokenEndpoint || oidc.token_endpoint;
  }
  const tokenExchangeEndpoint = info.TokenExchangeEndpoint || info.token_exchange_endpoint || `${base}/auth/exchange`;
  if (!authorizationEndpoint) {
    throw new Error('authorization endpoint missing from ConfigHub discovery');
  }
  if (!tokenEndpoint) {
    throw new Error('OIDC token endpoint missing from ConfigHub discovery');
  }
  if (!tokenExchangeEndpoint) {
    throw new Error('TokenExchangeEndpoint missing from ConfigHub discovery');
  }
  return {
    authIssuer,
    authorizationEndpoint,
    tokenEndpoint,
    tokenExchangeEndpoint,
  };
}

export async function startSignIn(config) {
  if (!config.oauthClientId) {
    throw new Error('OAUTH_CLIENT_ID is required for browser OAuth');
  }
  const endpoints = await discover(config);
  const verifier = randomString(48);
  const state = randomString(24);
  const challenge = await sha256(verifier);
  sessionStorage.setItem(VERIFIER_KEY, verifier);
  sessionStorage.setItem(STATE_KEY, state);
  const url = new URL(endpoints.authorizationEndpoint);
  url.searchParams.set('response_type', 'code');
  url.searchParams.set('client_id', config.oauthClientId);
  url.searchParams.set('redirect_uri', callbackUrl(config));
  url.searchParams.set('scope', 'openid profile email organization');
  url.searchParams.set('state', state);
  url.searchParams.set('code_challenge', challenge);
  url.searchParams.set('code_challenge_method', 'S256');
  location.assign(url.toString());
}

export async function completeSignIn(config) {
  const params = new URLSearchParams(location.search);
  const code = params.get('code');
  const state = params.get('state');
  if (!code) {
    return null;
  }
  if (state !== sessionStorage.getItem(STATE_KEY)) {
    throw new Error('OAuth state mismatch');
  }
  const verifier = sessionStorage.getItem(VERIFIER_KEY);
  const endpoints = await discover(config);
  const identityResponse = await fetch(endpoints.tokenEndpoint, {
    method: 'POST',
    headers: {'content-type': 'application/x-www-form-urlencoded'},
    body: new URLSearchParams({
      grant_type: 'authorization_code',
      code,
      redirect_uri: callbackUrl(config),
      client_id: config.oauthClientId,
      code_verifier: verifier,
    }),
  });
  if (!identityResponse.ok) {
    throw new Error('Identity token endpoint failed');
  }
  const identityToken = await identityResponse.json();
  const exchangeResponse = await fetch(endpoints.tokenExchangeEndpoint, {
    method: 'POST',
    headers: {'content-type': 'application/x-www-form-urlencoded'},
    body: new URLSearchParams({
      grant_type: 'urn:ietf:params:oauth:grant-type:token-exchange',
      subject_token: identityToken.access_token,
      subject_token_type: 'urn:ietf:params:oauth:token-type:access_token',
    }),
  });
  if (!exchangeResponse.ok) {
    throw new Error('TokenExchangeEndpoint failed');
  }
  const token = await exchangeResponse.json();
  const accessToken = token.access_token || token.AccessToken;
  if (!accessToken) {
    throw new Error('TokenExchangeEndpoint response did not include an access token');
  }
  sessionStorage.setItem(ACCESS_TOKEN_KEY, accessToken);
  history.replaceState({}, '', location.origin + location.pathname);
  return accessToken;
}

export function getAccessToken() {
  return sessionStorage.getItem(ACCESS_TOKEN_KEY);
}

export async function configHubFetch(config, path, options = {}) {
  const token = getAccessToken();
  if (!token) {
    throw new Error('Sign in before calling ConfigHub');
  }
  const base = (config.configHubBaseUrl || '').replace(/\/$/, '');
  const response = await fetch(`${base}${path}`, {
    ...options,
    headers: {
      ...(options.headers || {}),
      authorization: `Bearer ${token}`,
    },
  });
  if (!response.ok) {
    throw new Error(`ConfigHub API returned ${response.status}`);
  }
  return response.json();
}
