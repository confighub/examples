import { callConfigHub } from "./auth.js";

const KNOWN_ADDONS = [
  "cert-manager",
  "external-secrets",
  "ingress-nginx",
  "kyverno",
  "metrics-server",
  "postgresql",
  "prometheus",
  "redis",
];

function asList(payload) {
  if (Array.isArray(payload)) return payload;
  for (const key of ["items", "Items", "Spaces", "spaces", "Units", "units", "Revisions", "revisions", "Data", "data"]) {
    if (Array.isArray(payload?.[key])) return payload[key];
  }
  return [];
}

function first(object, keys, fallback = "") {
  for (const key of keys) {
    if (object?.[key] !== undefined && object[key] !== null) return object[key];
  }
  return fallback;
}

function parseLabels(value) {
  if (!value) return {};
  if (typeof value === "object" && !Array.isArray(value)) return {...value};
  if (typeof value !== "string") return {};
  const labels = {};
  for (const part of value.split(",")) {
    const index = part.indexOf("=");
    if (index > 0) labels[part.slice(0, index).trim()] = part.slice(index + 1).trim();
  }
  return labels;
}

function addonOf(spaceName) {
  const raw = String(spaceName || "").replace(/^helm-/, "");
  for (const addon of KNOWN_ADDONS) {
    if (raw === addon || raw.startsWith(`${addon}-`)) {
      return {addon, variant: raw.slice(addon.length).replace(/^-/, "") || "default"};
    }
  }
  const parts = raw.split("-").filter(Boolean);
  return {addon: parts[0] || raw || "unknown", variant: parts.slice(1).join("-") || "default"};
}

function normalizeSpace(item) {
  const space = item?.Space || item || {};
  const slug = first(space, ["Slug", "slug", "Name", "name", "DisplayName", "displayName"]);
  const parsed = addonOf(slug);
  return {
    id: first(space, ["SpaceID", "spaceID", "space_id", "id"], ""),
    slug,
    displayName: first(space, ["DisplayName", "displayName", "Name", "name"], slug),
    addon: first(space, ["Addon", "addon"], parsed.addon),
    variant: first(space, ["Variant", "variant"], parsed.variant),
    unitCount: Number(first(item, ["TotalUnitCount", "TotalUnits", "unitCount"], first(space, ["UnitCount", "unitCount"], 0))) || 0,
    raw: space,
  };
}

function normalizeUnit(item, fallbackVariant = "default") {
  const unit = item?.Unit || item || {};
  const labels = parseLabels(unit.Labels || unit.labels);
  return {
    id: first(unit, ["UnitID", "unitID", "unit_id", "id"], ""),
    slug: first(unit, ["Slug", "slug", "Name", "name", "DisplayName", "displayName"]),
    displayName: first(unit, ["DisplayName", "displayName", "Name", "name", "Slug", "slug"]),
    component: labels.Component || labels["app.kubernetes.io/name"] || labels.HelmChart || "",
    chartVersion: labels.HelmChartVersion || labels["helm.sh/chart"] || "",
    variant: labels.Variant || labels.variant || fallbackVariant,
    headRevision: first(unit, ["HeadRevisionNum", "headRevision", "HeadRevision"], ""),
    appliedRevision: first(unit, ["LastAppliedRevisionNum", "appliedRevision"], ""),
    liveRevision: first(unit, ["LiveRevisionNum", "liveRevision"], ""),
    raw: unit,
  };
}

function normalizeRevision(item) {
  const revision = item?.Revision || item || {};
  return {
    number: first(revision, ["RevisionNum", "Num", "number"], ""),
    createdAt: first(revision, ["CreatedAt", "createdAt"], ""),
    source: first(revision, ["Source", "source"], ""),
    description: first(revision, ["Description", "description"], ""),
    author: first(revision, ["CreatedBy", "UserID", "author"], ""),
  };
}

function parseUnitData(text = "") {
  const read = (pattern) => {
    const match = text.match(pattern);
    return match ? match[1].trim() : "";
  };
  return {
    chart: read(/helm\.sh\/chart:\s*"?([^"\n]+)"?/m),
    appVersion: read(/app\.kubernetes\.io\/version:\s*"?([^"\n]+)"?/m),
    image: read(/^\s*-?\s*image:\s*"?([^"\n]+)"?/m),
    replicas: read(/^\s*replicas:\s*"?([^"\n]+)"?/m),
    serviceAccount: read(/^\s*serviceAccountName:\s*"?([^"\n]+)"?/m),
  };
}

function buildInventory(spaces) {
  const groups = new Map();
  for (const space of spaces) {
    const group = groups.get(space.addon) || {addon: space.addon, variants: []};
    group.variants.push(space);
    groups.set(space.addon, group);
  }
  const addons = [...groups.values()].sort((a, b) => a.addon.localeCompare(b.addon));
  for (const group of addons) group.variants.sort((a, b) => a.variant.localeCompare(b.variant));
  return {
    source: "browser-oauth",
    totals: {
      addons: addons.length,
      variants: spaces.length,
      units: addons.reduce((sum, group) => sum + group.variants.reduce((inner, variant) => inner + Number(variant.unitCount || 0), 0), 0),
    },
    addons,
  };
}

function approvalScope(detail) {
  return {
    action: "review-addon-version",
    app: "Add-on Manager",
    addon: detail.space?.addon || "unknown",
    variant: detail.space?.variant || detail.unit?.variant || "unknown",
    space: detail.space?.slug || "unknown",
    unit: detail.unit?.slug || "unknown",
    currentChart: detail.parsedData?.chart || "",
    currentAppVersion: detail.parsedData?.appVersion || "",
    currentImage: detail.parsedData?.image || "",
    requiredFields: ["org", "space", "variant", "unit", "action", "revision", "target", "strategy", "addon", "version"],
    blockedUntil: [
      "approval object is created",
      "apply route is connected",
      "controller delivery proof is available",
      "runtime readback proof is available",
    ],
  };
}

function receipt(detail) {
  return {
    status: "preview-only",
    app: "Add-on Manager",
    scope: approvalScope(detail),
    proof: {
      revision: detail.revisions?.[0] ? "revision history available" : "revision history unavailable",
      approval: "not created by this sample",
      gate: "mutation route blocked",
      controller: "not connected",
      runtime: "not connected",
      receipt: "preview generated in the browser",
    },
    nonClaims: [
      "No live ConfigHub mutation was performed.",
      "No controller reconciliation was proven.",
      "No runtime Kubernetes state was proven.",
    ],
  };
}

export function createConfigHubBrowserClient(session) {
  async function request(path, options = {}) {
    return callConfigHub(session, path, options);
  }

  async function inventory() {
    const spacePayload = await request("/api/space");
    const spaces = asList(spacePayload)
      .map(normalizeSpace)
      .filter((space) => space.slug?.startsWith("helm-") && space.id);
    await Promise.all(spaces.map(async (space) => {
      const unitPayload = await request(`/api/space/${encodeURIComponent(space.id)}/unit`);
      const units = asList(unitPayload).map((unit) => normalizeUnit(unit, space.variant));
      space.units = units;
      space.unitCount = units.length || space.unitCount;
    }));
    return buildInventory(spaces);
  }

  async function me() {
    return request("/api/me");
  }

  async function detail(space, unit) {
    const [unitData, revisionPayload] = await Promise.all([
      request(`/api/space/${encodeURIComponent(space.id)}/unit/${encodeURIComponent(unit.id)}/data`, {asText: true}),
      request(`/api/space/${encodeURIComponent(space.id)}/unit/${encodeURIComponent(unit.id)}/revision`),
    ]);
    const parsedData = parseUnitData(unitData);
    const latestImage = Boolean(parsedData.image && /:latest\s*$/.test(parsedData.image));
    return {
      source: "browser-oauth",
      space,
      unit,
      parsedData,
      warnings: latestImage ? ["Image tag is :latest and should be pinned before rollout."] : [],
      revisions: asList(revisionPayload).map(normalizeRevision),
      unitData,
    };
  }

  return {
    me,
    inventory,
    detail,
    proposal: (detailPayload) => ({source: "browser-oauth", approvalScope: approvalScope(detailPayload), previewOnly: true}),
    receipt,
  };
}
