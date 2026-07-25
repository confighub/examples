import { describe, expect, it } from 'vitest';

import { appendPanel, parseDashboard, serializePanelItem } from './parse';
import type { Panel } from './types';

const DOC = `apiVersion: configboard.confighub.com/v1
kind: Dashboard
slug: demo
title: Demo

# This comment explains why the panel below is shaped the way it is.
panels:
  - id: first
    title: First
    query:
      source: Space
    transform:
      aggregate: { fn: sum, field: Space.TotalUnitCount }
    chart: { form: statTile }
`;

const PANEL: Panel = {
  id: 'added',
  title: 'Added by the builder',
  span: 6,
  query: { source: 'Unit' },
  transform: { groupBy: 'Unit.ToolchainType', aggregate: { fn: 'count' }, dropEmpty: true },
  chart: { form: 'bar', orientation: 'horizontal' },
};

describe('serializePanelItem', () => {
  it('emits a list item indented to sit under panels:', () => {
    const item = serializePanelItem(PANEL);
    expect(item.startsWith('  - ')).toBe(true);
    // Every continuation line is indented to the item body, not the list marker.
    for (const line of item.split('\n').slice(1)) {
      expect(line.startsWith('    ')).toBe(true);
    }
  });
});

describe('appendPanel', () => {
  it('produces a document that still parses, with the panel added', () => {
    const next = appendPanel(DOC, PANEL);
    const { dashboard, errors } = parseDashboard(next);
    expect(errors).toEqual([]);
    expect(dashboard!.panels.map((p) => p.id)).toEqual(['first', 'added']);
  });

  it('preserves the existing panels exactly', () => {
    const before = parseDashboard(DOC).dashboard!;
    const after = parseDashboard(appendPanel(DOC, PANEL)).dashboard!;
    expect(after.panels[0]).toEqual(before.panels[0]);
  });

  it('preserves comments', () => {
    // Re-serializing the whole document would drop them, which is why the builder
    // appends text rather than rewriting.
    expect(appendPanel(DOC, PANEL)).toContain('# This comment explains why');
  });

  it('round-trips the appended panel', () => {
    const after = parseDashboard(appendPanel(DOC, PANEL)).dashboard!;
    const added = after.panels.find((p) => p.id === 'added');
    expect(added).toEqual(PANEL);
  });

  it('appends twice without corrupting the document', () => {
    const twice = appendPanel(appendPanel(DOC, PANEL), { ...PANEL, id: 'second', title: 'Second' });
    const { dashboard, errors } = parseDashboard(twice);
    expect(errors).toEqual([]);
    expect(dashboard!.panels.map((p) => p.id)).toEqual(['first', 'added', 'second']);
  });

  it('tolerates a document with trailing whitespace', () => {
    const { errors } = parseDashboard(appendPanel(`${DOC}\n\n  \n`, PANEL));
    expect(errors).toEqual([]);
  });
});
