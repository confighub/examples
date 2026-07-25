// M0 ships the starter dashboards bundled. They are the same YAML documents that
// M1 will store as Units in the configboard Space — the loader changes, the document
// does not.

import deliveryHealth from '../../../dashboards/delivery-health.yaml?raw';
import fleetOverview from '../../../dashboards/fleet-overview.yaml?raw';
import fleetPosture from '../../../dashboards/fleet-posture.yaml?raw';
import resourceInventory from '../../../dashboards/resource-inventory.yaml?raw';
import versionSkew from '../../../dashboards/version-skew.yaml?raw';
import { parseDashboard } from '../model/parse';
import type { Dashboard } from '../model/types';

export interface LoadedDashboard {
  dashboard: Dashboard;
  errors: string[];
  /** The document text, so it can be seeded into ConfigHub verbatim. */
  yaml: string;
}

const SOURCES = [fleetOverview, versionSkew, resourceInventory, fleetPosture, deliveryHealth];

export function bundledDashboards(): LoadedDashboard[] {
  const loaded: LoadedDashboard[] = [];
  for (const text of SOURCES) {
    const { dashboard, errors } = parseDashboard(text);
    if (dashboard) loaded.push({ dashboard, errors, yaml: text });
  }
  return loaded;
}
