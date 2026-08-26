// Configuration data access. A Unit's configuration is not a field of the Unit: the
// entity carries DataHash and DataSize, and the document itself is read from and
// written to its own endpoints as application/octet-stream. @confighub/api ships the
// helpers that get the parsing and serialization right; these add the two things every
// example app wanted on top — throw-on-error text results, and a bulk read.

import {
  getRevisionData,
  getUnitData,
  putUnitData,
  type components,
} from '@confighub/api';

import { confighub } from './client';

export type UnitData = components['schemas']['UnitData'];

/** A Unit's configuration, as text. Throws if it cannot be read. */
export async function fetchUnitDataText(spaceId: string, unitId: string): Promise<string> {
  const { data, response } = await getUnitData(confighub(), { spaceId, unitId });
  if (data === undefined) {
    throw new Error(`unit ${unitId} data: HTTP ${response.status}`);
  }
  return data;
}

/** One Revision's configuration, as text. Throws if it cannot be read. */
export async function fetchRevisionDataText(
  spaceId: string,
  unitId: string,
  revisionId: string,
): Promise<string> {
  const { data, response } = await getRevisionData(confighub(), { spaceId, unitId, revisionId });
  if (data === undefined) {
    throw new Error(`revision ${revisionId} data: HTTP ${response.status}`);
  }
  return data;
}

/**
 * Replace a Unit's configuration. An empty string is a real configuration — emptying a
 * Unit is how its resources are withdrawn — so callers must not guard this on the text
 * being non-empty.
 */
export async function putUnitDataText(
  spaceId: string,
  unitId: string,
  data: string,
  changeDescription?: string,
): Promise<void> {
  const { response, error } = await putUnitData(confighub(), { spaceId, unitId }, data, {
    lastChangeDescription: changeDescription,
  });
  if (error !== undefined || !response.ok) {
    throw new Error(`unit ${unitId} data: HTTP ${response.status}`);
  }
}

export interface ListUnitDataOptions {
  /** Filter expression over Units, the same `where` syntax the CLI uses. */
  where?: string;
  /** Filter expression evaluated against each Unit's configuration. */
  whereData?: string;
  /** Fields to return on the Unit. */
  select?: string;
  /** Related entities to expand. */
  include?: string;
}

/**
 * Every matching Unit's configuration in one request. A view over many Units should
 * reach for this rather than a read per Unit: the per-Unit loop is the difference
 * between one round trip and a thousand, and it is the reason a fleet page is slow.
 */
export async function listUnitData(options: ListUnitDataOptions = {}): Promise<UnitData[]> {
  const { data, error, response } = await confighub().GET('/unit_data', {
    params: {
      query: {
        where: options.where,
        where_data: options.whereData,
        select: options.select,
        include: options.include,
      },
    },
  });
  if (error !== undefined || data === undefined) {
    throw new Error(`GET /unit_data: HTTP ${response.status}`);
  }
  return data;
}
