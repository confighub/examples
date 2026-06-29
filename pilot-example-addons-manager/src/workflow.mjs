export const WORKFLOW = {
  app: "Add-on Manager",
  route: "ConfigHub -> OCI -> Controller -> Runtime",
  summary: "Review platform add-ons across Variants, preview safe changes, and show proof gaps before any rollout.",
  steps: [
    {id: "map", label: "Map inventory", purpose: "Find add-ons, Variants, Units, and current versions."},
    {id: "preview", label: "Preview configuration", purpose: "Read current ConfigHub Unit data and revision state."},
    {id: "scope", label: "Scope approval", purpose: "Name the exact org, space, Variant, Unit, action, and version."},
    {id: "apply", label: "Apply rollout", purpose: "Blocked in this sample until a governed write path is connected."},
    {id: "prove", label: "Prove result", purpose: "Show revision, approval, controller, runtime, and receipt evidence."},
  ],
  scopeFields: ["org", "space", "variant", "unit", "action", "revision", "target", "strategy", "addon", "version"],
  proofTabs: ["Revision", "Approval", "Gate", "Controller", "Runtime", "Receipt"],
  blockedActions: [
    "Create approval",
    "Apply rollout",
    "Change live ConfigHub Unit data",
    "Claim controller or runtime success",
  ],
};

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

export function asArray(payload, keys = ["items", "Items", "spaces", "Spaces", "units", "Units", "revisions", "Revisions"]) {
  if (Array.isArray(payload)) return payload;
  for (const key of keys) {
    if (payload && Array.isArray(payload[key])) return payload[key];
  }
  return [];
}

export function readFirst(object, keys, fallback = "") {
  for (const key of keys) {
    if (object && object[key] !== undefined && object[key] !== null) return object[key];
  }
  return fallback;
}

export function parseLabels(value) {
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

export function addonOfSpace(spaceName) {
  const raw = String(spaceName || "").replace(/^helm-/, "");
  for (const addon of KNOWN_ADDONS) {
    if (raw === addon || raw.startsWith(`${addon}-`)) {
      return {addon, variant: raw.slice(addon.length).replace(/^-/, "") || "default"};
    }
  }
  const parts = raw.split("-").filter(Boolean);
  return {
    addon: parts[0] || raw || "unknown",
    variant: parts.slice(1).join("-") || "default",
  };
}

export function normalizeSpace(item) {
  const space = item?.Space || item || {};
  const name = readFirst(space, ["Slug", "slug", "Name", "name", "DisplayName", "displayName"]);
  const parsed = addonOfSpace(name);
  return {
    slug: name,
    displayName: readFirst(space, ["DisplayName", "displayName", "Name", "name"], name),
    addon: readFirst(space, ["Addon", "addon"], parsed.addon),
    variant: readFirst(space, ["Variant", "variant"], parsed.variant),
    unitCount: Number(readFirst(item, ["TotalUnitCount", "TotalUnits", "unitCount"], readFirst(space, ["UnitCount", "unitCount"], 0))) || 0,
  };
}

export function normalizeUnit(item, fallbackVariant = "default") {
  const unit = item?.Unit || item || {};
  const labels = parseLabels(unit.Labels || unit.labels);
  return {
    slug: readFirst(unit, ["Slug", "slug", "Name", "name", "DisplayName", "displayName"]),
    displayName: readFirst(unit, ["DisplayName", "displayName", "Name", "name", "Slug", "slug"]),
    component: labels.Component || labels["app.kubernetes.io/name"] || labels.HelmChart || "",
    chartVersion: labels.HelmChartVersion || labels["helm.sh/chart"] || "",
    variant: labels.Variant || labels.variant || fallbackVariant,
    headRevision: readFirst(unit, ["HeadRevisionNum", "headRevision", "HeadRevision"], ""),
    appliedRevision: readFirst(unit, ["LastAppliedRevisionNum", "appliedRevision"], ""),
    liveRevision: readFirst(unit, ["LiveRevisionNum", "liveRevision"], ""),
    labels,
    raw: unit,
  };
}

export function normalizeRevision(item) {
  const revision = item?.Revision || item || {};
  return {
    number: readFirst(revision, ["RevisionNum", "Num", "number"], ""),
    createdAt: readFirst(revision, ["CreatedAt", "createdAt"], ""),
    source: readFirst(revision, ["Source", "source"], ""),
    description: readFirst(revision, ["Description", "description"], ""),
    author: readFirst(revision, ["CreatedBy", "author"], ""),
  };
}

export function parseUnitData(text = "") {
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

export function buildInventory(spacesPayload, unitsBySpace = {}) {
  const spaces = asArray(spacesPayload).map(normalizeSpace).filter((space) => space.slug?.startsWith("helm-"));
  const groups = new Map();
  for (const space of spaces) {
    const units = asArray(unitsBySpace[space.slug] || [], ["items", "Units", "units"]).map((unit) => normalizeUnit(unit, space.variant));
    const group = groups.get(space.addon) || {addon: space.addon, variants: []};
    group.variants.push({...space, units, unitCount: units.length || space.unitCount});
    groups.set(space.addon, group);
  }
  const addons = [...groups.values()].sort((a, b) => a.addon.localeCompare(b.addon));
  for (const group of addons) {
    group.variants.sort((a, b) => a.variant.localeCompare(b.variant));
  }
  return {
    totals: {
      addons: addons.length,
      variants: spaces.length,
      units: addons.reduce((sum, group) => sum + group.variants.reduce((subtotal, variant) => subtotal + Number(variant.unitCount || 0), 0), 0),
    },
    addons,
  };
}

export function buildUnitDetail({space, unit, unitRecord, revisions, unitData, source}) {
  const normalized = normalizeUnit(unitRecord, space?.variant || "default");
  const parsedData = parseUnitData(unitData || "");
  const latestImage = Boolean(parsedData.image && /:latest\s*$/.test(parsedData.image));
  return {
    source,
    space,
    unit: normalized,
    parsedData,
    warnings: latestImage ? ["Image tag is :latest and should be pinned before rollout."] : [],
    revisions: asArray(revisions, ["items", "revisions", "Revisions"]).map(normalizeRevision),
    unitData: unitData || "",
  };
}

export function buildApprovalScope(detail, action = "review-addon-version") {
  return {
    action,
    app: WORKFLOW.app,
    addon: detail.space?.addon || "unknown",
    variant: detail.space?.variant || detail.unit?.variant || "unknown",
    space: detail.space?.slug || "unknown",
    unit: detail.unit?.slug || "unknown",
    currentChart: detail.parsedData?.chart || "",
    currentAppVersion: detail.parsedData?.appVersion || "",
    currentImage: detail.parsedData?.image || "",
    requiredFields: WORKFLOW.scopeFields,
    blockedUntil: [
      "approval object is created",
      "apply route is connected",
      "controller delivery proof is available",
      "runtime readback proof is available",
    ],
  };
}

export function buildReceipt(detail) {
  return {
    status: "preview-only",
    app: WORKFLOW.app,
    scope: buildApprovalScope(detail),
    proof: {
      revision: detail.revisions?.[0] ? "revision history available" : "revision history unavailable",
      approval: "not created by this sample",
      gate: "mutation route blocked",
      controller: "not connected",
      runtime: "not connected",
      receipt: "preview generated locally",
    },
    nonClaims: [
      "No live ConfigHub mutation was performed.",
      "No controller reconciliation was proven.",
      "No runtime Kubernetes state was proven.",
    ],
  };
}
