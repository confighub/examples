// Raw-text fetcher for the unit data endpoint. The generated client JSON-parses every
// response, but /data returns plain YAML — JSON.parse throws and the hook yields no
// data. This goes through fetch directly, with the same bearer token the RTK client
// uses (read from @confighub/react-auth's non-React accessor).

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
