import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

import { parseDashboard, serializeDashboard } from './parse';

const DASHBOARD_DIR = join(__dirname, '../../../dashboards');

// A dashboard is stored as its YAML document, so the round trip has to hold: parse a
// document, serialize it, parse it again, and the model must be the same. Otherwise
// saving a dashboard through the app would quietly change what it does.
describe('serializeDashboard round trip', () => {
  const files = readdirSync(DASHBOARD_DIR).filter((f) => f.endsWith('.yaml'));

  for (const file of files) {
    it(`${file} survives parse -> serialize -> parse`, () => {
      const original = parseDashboard(readFileSync(join(DASHBOARD_DIR, file), 'utf8'));
      expect(original.dashboard).toBeDefined();

      const round = parseDashboard(serializeDashboard(original.dashboard!));
      expect(round.errors).toEqual([]);
      expect(round.dashboard).toEqual(original.dashboard);
    });
  }

  it('emits a document the parser accepts from a minimal model', () => {
    const yaml = serializeDashboard({
      apiVersion: 'configboard.confighub.com/v1',
      kind: 'Dashboard',
      slug: 'minimal',
      title: 'Minimal',
      panels: [
        {
          id: 'p',
          title: 'Units',
          query: { source: 'Unit' },
          transform: { aggregate: { fn: 'count' } },
          chart: { form: 'statTile' },
        },
      ],
    });

    const { dashboard, errors } = parseDashboard(yaml);
    expect(errors).toEqual([]);
    expect(dashboard?.panels).toHaveLength(1);
  });

  it('omits absent optional fields rather than emitting nulls', () => {
    const yaml = serializeDashboard({
      apiVersion: 'configboard.confighub.com/v1',
      kind: 'Dashboard',
      slug: 'no-extras',
      title: 'No extras',
      panels: [
        { id: 'p', title: 'p', query: { source: 'Space' }, chart: { form: 'statTile' } },
      ],
    });
    expect(yaml).not.toContain('description:');
    expect(yaml).not.toContain('variables:');
    expect(yaml).not.toContain('null');
  });
});
