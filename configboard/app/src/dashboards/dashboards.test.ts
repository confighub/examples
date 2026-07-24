// The shipped dashboards are data, so they can be wrong in the ways data is wrong:
// a typo'd dimension, a chart form that does not exist, a groupBy the source cannot
// answer. This test parses the real YAML files and holds them to the registry.

import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';
import { describe, expect, it } from 'vitest';

import { parseDashboard } from '../model/parse';
import type { Dashboard } from '../model/types';
import { compilePanel, referencedDimensions, unknownDimensions } from '../query/compile';
import { lookupDimension } from '../query/dimensions';

const DASHBOARD_DIR = join(__dirname, '../../../dashboards');

function loadAll(): { file: string; dashboard: Dashboard; errors: string[] }[] {
  return readdirSync(DASHBOARD_DIR)
    .filter((f) => f.endsWith('.yaml'))
    .map((file) => {
      const { dashboard, errors } = parseDashboard(readFileSync(join(DASHBOARD_DIR, file), 'utf8'));
      if (!dashboard) throw new Error(`${file} failed to parse: ${errors.join('; ')}`);
      return { file, dashboard, errors };
    });
}

const dashboards = loadAll();

describe('bundled dashboards', () => {
  it('ships more than one', () => {
    expect(dashboards.length).toBeGreaterThan(1);
  });

  for (const { file, dashboard, errors } of dashboards) {
    describe(file, () => {
      it('parses without errors', () => {
        expect(errors).toEqual([]);
      });

      it('has a unique slug and panel ids', () => {
        const ids = dashboard.panels.map((p) => p.id);
        expect(new Set(ids).size).toBe(ids.length);
        expect(dashboard.slug).toMatch(/^[a-z0-9-]+$/);
      });

      it('names only known dimensions', () => {
        for (const panel of dashboard.panels) {
          expect({ panel: panel.id, unknown: unknownDimensions(panel) }).toEqual({
            panel: panel.id,
            unknown: [],
          });
        }
      });

      it('groups only by dimensions its source can answer', () => {
        for (const panel of dashboard.panels) {
          for (const id of referencedDimensions(panel.transform)) {
            const dim = lookupDimension(panel.query.source, id);
            expect(dim, `${panel.id} -> ${id} on ${panel.query.source}`).toBeDefined();
          }
        }
      });

      it('declares every variable its panels reference', () => {
        const declared = new Set((dashboard.variables ?? []).map((v) => v.name));
        for (const panel of dashboard.panels) {
          const refs = [...(panel.query.where ?? '').matchAll(/\$\{(\w+)/g)].map((m) => m[1]);
          for (const ref of refs) {
            expect(declared.has(ref), `${panel.id} references \${${ref}}`).toBe(true);
          }
        }
      });

      it('compiles every panel to a request', () => {
        const scope: Record<string, string> = {};
        for (const v of dashboard.variables ?? []) scope[v.name] = v.default ?? '*';

        for (const panel of dashboard.panels) {
          const spec = compilePanel(panel, scope);
          expect(spec.source).toBe(panel.query.source);
          // Unit queries must never pull config bodies back.
          if (spec.source === 'Unit' && spec.select) {
            expect(spec.select).not.toMatch(/\bData\b|\bLiveData\b|\bLiveState\b/);
          }
        }
      });

      it('uses a meter only where a denominator is available', () => {
        for (const panel of dashboard.panels) {
          if (panel.chart.form === 'meter') {
            expect(panel.chart.totalField, `${panel.id} needs chart.totalField`).toBeDefined();
          }
        }
      });
    });
  }
});
