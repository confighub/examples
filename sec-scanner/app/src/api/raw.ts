// Raw-text access to the data endpoints. Configuration is not a field of a Unit
// or a Revision: it is read and written through their own /data endpoints, as
// text. The generated RTK client JSON-parses every response, but a /data body is
// the configuration itself — JSON.parse throws and the hook yields no data. These
// go through fetch directly with the same auth (cookies + optional bearer token).

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

export async function putUnitDataText(
  spaceId: string,
  unitId: string,
  data: string,
  changeDescription?: string,
): Promise<void> {
  const token = getStoredToken();
  const query = changeDescription
    ? `?last_change_description=${encodeURIComponent(changeDescription)}`
    : '';
  const path = `/api/space/${spaceId}/unit/${unitId}/data${query}`;
  const response = await fetch(path, {
    method: 'PUT',
    credentials: 'include',
    headers: {
      'Content-Type': 'application/octet-stream',
      ...(token !== null ? { Authorization: `Bearer ${token}` } : {}),
    },
    body: data,
  });
  if (!response.ok) {
    throw new Error(`${path}: HTTP ${response.status}`);
  }
}
