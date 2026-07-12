const MESSAGE = 'Use the ConfigHub Custom UI Apps JavaScript SDK surface: run npm install, npm run oauth:register, then VITE_CONFIGHUB_BASE_URL=https://hub.confighub.com VITE_OAUTH_CLIENT_ID=<client-id> npm run ui:dev.';

export async function startSignIn() {
  throw new Error(MESSAGE);
}

export async function completeSignIn() {
  return null;
}

export function getAccessToken() {
  return null;
}

export async function configHubFetch() {
  throw new Error(MESSAGE);
}
