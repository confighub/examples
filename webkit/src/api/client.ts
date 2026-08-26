// The typed ConfigHub client for code that runs outside React — snapshot loaders, data
// reads, harnesses. Inside a component `useConfigHub()` returns the same shape; this
// exists so a plain function does not have to be handed one.
//
// In the browser the token comes from @confighub/react-auth's non-React accessor, so
// there is one session in the app no matter which client a call goes through. A harness
// running under Node has no such session and installs its own client with
// configureClient() before anything else imports this.

import { createConfigHubClient, type ConfigHubClient, type ConfigHubClientOptions } from '@confighub/api';
import { getAccessToken } from '@confighub/react-auth';

import { BASE_URL } from '../config';

let client: ConfigHubClient | null = null;

/**
 * Replace the shared client. For non-browser callers (validation harnesses, scripts),
 * which have a token from somewhere else — typically `cub auth get-token` — and no
 * instance URL from the Vite env. Call it before importing anything that reads data.
 */
export function configureClient(options: ConfigHubClientOptions): void {
  client = createConfigHubClient(options);
}

/** The shared client. Created against the browser session on first use if unset. */
export function confighub(): ConfigHubClient {
  client ??= createConfigHubClient({ baseUrl: BASE_URL, getToken: getAccessToken });
  return client;
}
