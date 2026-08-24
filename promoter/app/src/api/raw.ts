// Raw-text access to the data endpoints. Configuration is not a field of a Unit or a
// Revision: it is read and written through their own /data endpoints, as text. The
// generated RTK client JSON-parses every response, but a /data body is the
// configuration itself — JSON.parse throws and the hook yields no data. These go
// through fetch directly, browser-direct to the ConfigHub instance with the same bearer
// token the RTK client uses (read from @confighub/react-auth's non-React accessor).

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

export async function putUnitDataText(
  spaceId: string,
  unitId: string,
  data: string,
  changeDescription?: string,
): Promise<void> {
  const token = getAccessToken();
  const query = changeDescription
    ? `?last_change_description=${encodeURIComponent(changeDescription)}`
    : '';
  const path = `/space/${spaceId}/unit/${unitId}/data`;
  const response = await fetch(`${API_BASE}${path}${query}`, {
    method: 'PUT',
    headers: {
      'Content-Type': 'application/octet-stream',
      ...(token ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: data,
  });
  if (!response.ok) {
    throw new Error(`${path}: HTTP ${response.status}`);
  }
}
