// Raw-text access to the unit data endpoint. Configuration is not a field of a Unit:
// it is read and written through the Unit's own /data endpoint, as text. The generated
// client JSON-parses every response, but /data's body is the configuration itself —
// JSON.parse throws and the hook yields no data. These go through fetch directly, with
// the same bearer token the RTK client uses (read from @confighub/react-auth's
// non-React accessor).

import { getAccessToken } from '@confighub/react-auth';

import { BASE_URL } from '../app/config';

const API_BASE = `${BASE_URL.replace(/\/+$/, '')}/api`;

export async function fetchUnitDataText(spaceId: string, unitId: string): Promise<string> {
  const token = getAccessToken();
  const path = `/space/${spaceId}/unit/${unitId}/data`;
  const response = await fetch(`${API_BASE}${path}`, {
    headers: token ? { Authorization: `Bearer ${token}` } : undefined,
  });
  if (!response.ok) {
    throw new Error(`${path}: HTTP ${response.status}`);
  }
  return response.text();
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
