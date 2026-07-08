// Raw-text fetchers for the data-download endpoints. The generated RTK client
// JSON-parses every response, but /data endpoints return plain YAML — JSON.parse
// throws and the hook yields no data. These go through fetch directly, browser-
// direct to the ConfigHub instance with the same bearer token the RTK client uses
// (read from @confighub/react-auth's non-React accessor).

import { getAccessToken } from '@confighub/react-auth';

// Mirror @confighub/rtk-query's apiBaseUrl(): the instance origin, with `/api`
// appended unless already present.
const API_BASE = ((): string => {
  const raw = (import.meta.env.VITE_CONFIGHUB_BASE_URL ?? '').trim().replace(/\/+$/, '');
  return raw.endsWith('/api') ? raw : `${raw}/api`;
})();

async function fetchText(path: string): Promise<string> {
  const token = getAccessToken();
  const response = await fetch(`${API_BASE}${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (!response.ok) {
    throw new Error(`${path}: HTTP ${response.status}`);
  }
  return response.text();
}

export function fetchUnitDataText(spaceId: string, unitId: string): Promise<string> {
  return fetchText(`/space/${spaceId}/unit/${unitId}/data`);
}

export function fetchRevisionDataText(
  spaceId: string,
  unitId: string,
  revisionId: string,
): Promise<string> {
  return fetchText(`/space/${spaceId}/unit/${unitId}/revision/${revisionId}/data`);
}
