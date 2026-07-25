// Dashboard YAML -> Dashboard, with the validation errors a hand-edited document
// actually produces. Parsing never throws: a bad document reports its problems so the
// app can show them next to the dashboard rather than blanking the page.

import { parse as parseYaml, stringify as stringifyYaml } from 'yaml';

import type { ChartForm, Dashboard, Panel, SourceName, Variable } from './types';

const SOURCES: SourceName[] = ['Unit', 'Space', 'Revision', 'Target', 'Resource'];
const FORMS: ChartForm[] = [
  'statTile',
  'meter',
  'bar',
  'stackedBar',
  'line',
  'donut',
  'heatmap',
  'histogram',
  'table',
];

export interface ParseResult {
  dashboard?: Dashboard;
  errors: string[];
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === 'object' && value !== null && !Array.isArray(value);
}

function parsePanel(raw: unknown, index: number, errors: string[]): Panel | undefined {
  if (!isRecord(raw)) {
    errors.push(`panel ${index}: not an object`);
    return undefined;
  }

  const id = typeof raw.id === 'string' ? raw.id : `panel-${index}`;
  const title = typeof raw.title === 'string' ? raw.title : id;

  const query = isRecord(raw.query) ? raw.query : undefined;
  const source = query?.source;
  if (typeof source !== 'string' || !SOURCES.includes(source as SourceName)) {
    errors.push(`panel ${id}: query.source must be one of ${SOURCES.join(', ')}`);
    return undefined;
  }

  const chart = isRecord(raw.chart) ? raw.chart : undefined;
  const form = chart?.form;
  if (typeof form !== 'string' || !FORMS.includes(form as ChartForm)) {
    errors.push(`panel ${id}: chart.form must be one of ${FORMS.join(', ')}`);
    return undefined;
  }

  return {
    id,
    title,
    description: typeof raw.description === 'string' ? raw.description : undefined,
    span: typeof raw.span === 'number' ? raw.span : 6,
    query: query as unknown as Panel['query'],
    transform: isRecord(raw.transform) ? (raw.transform as unknown as Panel['transform']) : undefined,
    chart: chart as unknown as Panel['chart'],
  };
}

function parseVariable(raw: unknown, index: number, errors: string[]): Variable | undefined {
  if (!isRecord(raw) || typeof raw.name !== 'string') {
    errors.push(`variable ${index}: needs a name`);
    return undefined;
  }
  return {
    name: raw.name,
    label: typeof raw.label === 'string' ? raw.label : raw.name,
    type: raw.type === 'timeRange' ? 'timeRange' : 'select',
    from: isRecord(raw.from) ? (raw.from as unknown as Variable['from']) : undefined,
    default: typeof raw.default === 'string' ? raw.default : undefined,
    allValue: raw.allValue === true,
  };
}

export function parseDashboard(text: string): ParseResult {
  const errors: string[] = [];

  let raw: unknown;
  try {
    raw = parseYaml(text);
  } catch (e) {
    return { errors: [`YAML parse failed: ${e instanceof Error ? e.message : String(e)}`] };
  }

  if (!isRecord(raw)) return { errors: ['document is not a mapping'] };
  if (raw.kind !== 'Dashboard') errors.push(`kind must be "Dashboard", got ${String(raw.kind)}`);

  const slug = typeof raw.slug === 'string' ? raw.slug : undefined;
  if (!slug) errors.push('slug is required');

  const panelsRaw = Array.isArray(raw.panels) ? raw.panels : [];
  if (panelsRaw.length === 0) errors.push('dashboard has no panels');

  const panels = panelsRaw
    .map((p, i) => parsePanel(p, i, errors))
    .filter((p): p is Panel => p !== undefined);

  const variables = (Array.isArray(raw.variables) ? raw.variables : [])
    .map((v, i) => parseVariable(v, i, errors))
    .filter((v): v is Variable => v !== undefined);

  if (!slug) return { errors };

  return {
    dashboard: {
      apiVersion: typeof raw.apiVersion === 'string' ? raw.apiVersion : API_VERSION,
      kind: 'Dashboard',
      slug,
      title: typeof raw.title === 'string' ? raw.title : slug,
      description: typeof raw.description === 'string' ? raw.description : undefined,
      variables,
      panels,
    },
    errors,
  };
}

export const API_VERSION = 'configboard.confighub.com/v1';

/**
 * Dashboard -> YAML. The round trip is lossy by design: it emits the fields the model
 * knows about, so a saved document is normalized rather than byte-preserved. Comments
 * in a hand-edited document do not survive a save — which is why the source editor
 * writes back what the user typed, and only serialization of *app-built* dashboards
 * goes through here.
 */
export function serializeDashboard(dashboard: Dashboard): string {
  const doc: Record<string, unknown> = {
    apiVersion: dashboard.apiVersion || API_VERSION,
    kind: 'Dashboard',
    slug: dashboard.slug,
    title: dashboard.title,
  };
  if (dashboard.description) doc.description = dashboard.description;
  if (dashboard.variables && dashboard.variables.length > 0) doc.variables = dashboard.variables;
  doc.panels = dashboard.panels;

  return stringifyYaml(doc, { lineWidth: 0 });
}

/**
 * A single panel as a YAML list item, indented to sit under `panels:`.
 *
 * Appending text is deliberate: re-serializing the whole document would drop the
 * comments a hand-edited dashboard carries, and those comments are often the only record
 * of why a panel is shaped the way it is.
 */
export function serializePanelItem(panel: Panel): string {
  const body = stringifyYaml(panel, { lineWidth: 0 }).trimEnd();
  const lines = body.split('\n');
  return lines.map((line, i) => (i === 0 ? `  - ${line}` : `    ${line}`)).join('\n');
}

/** Appends a panel to a dashboard document's text, preserving everything already there. */
export function appendPanel(yaml: string, panel: Panel): string {
  const trimmed = yaml.replace(/\s+$/, '');
  return `${trimmed}\n\n${serializePanelItem(panel)}\n`;
}
