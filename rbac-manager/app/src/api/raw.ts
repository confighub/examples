// Raw-text fetchers for the data-download endpoints. The generated RTK
// client JSON-parses every response, but /data endpoints return plain YAML —
// JSON.parse throws and the hook yields no data. These go through fetch
// directly with the same auth (cookies + optional bearer token).

import { getStoredToken } from '../sdk/confighubapi';

async function fetchText(path: string): Promise<string> {
  const token = getStoredToken();
  const response = await fetch(path, {
    credentials: 'include',
    headers: token !== null ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (!response.ok) {
    throw new Error(`${path}: HTTP ${response.status}`);
  }
  return response.text();
}

export function fetchUnitDataText(spaceId: string, unitId: string): Promise<string> {
  return fetchText(`/api/space/${spaceId}/unit/${unitId}/data`);
}

export function fetchRevisionDataText(
  spaceId: string,
  unitId: string,
  revisionId: string,
): Promise<string> {
  return fetchText(`/api/space/${spaceId}/unit/${unitId}/revision/${revisionId}/data`);
}
