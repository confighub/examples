import assert from "node:assert/strict";
import test from "node:test";
import { fixtureDetail, fixtureInventory } from "../src/fixtures.mjs";
import { addonOfSpace, buildApprovalScope, buildReceipt, parseUnitData } from "../src/workflow.mjs";

test("space names map to add-ons and Variants", () => {
  assert.deepEqual(addonOfSpace("helm-metrics-server-prod"), {
    addon: "metrics-server",
    variant: "prod",
  });
  assert.deepEqual(addonOfSpace("helm-kyverno"), {
    addon: "kyverno",
    variant: "default",
  });
});

test("fixture inventory groups add-ons across Variants", async () => {
  const inventory = await fixtureInventory();
  assert.equal(inventory.source, "fixture");
  assert.equal(inventory.totals.addons, 3);
  assert.equal(inventory.totals.variants, 4);
  assert.equal(inventory.totals.units, 8);
  const metrics = inventory.addons.find((group) => group.addon === "metrics-server");
  assert.equal(metrics.variants.length, 2);
});

test("Unit data parser extracts chart, image, version, and replica fields", () => {
  const parsed = parseUnitData(`
metadata:
  labels:
    helm.sh/chart: metrics-server-3.12.1
    app.kubernetes.io/version: 0.7.1
spec:
  replicas: 2
  template:
    spec:
      containers:
        - image: example:v1
`);
  assert.equal(parsed.chart, "metrics-server-3.12.1");
  assert.equal(parsed.appVersion, "0.7.1");
  assert.equal(parsed.replicas, "2");
  assert.equal(parsed.image, "example:v1");
});

test("detail and receipt expose blocked proof gaps", async () => {
  const detail = await fixtureDetail("helm-kyverno-prod", "kyverno-controller");
  assert.equal(detail.source, "fixture");
  assert.equal(detail.warnings.length, 1);
  const scope = buildApprovalScope(detail);
  assert.equal(scope.space, "helm-kyverno-prod");
  assert.equal(scope.unit, "kyverno-controller");
  assert(scope.blockedUntil.includes("runtime readback proof is available"));
  const receipt = buildReceipt(detail);
  assert.equal(receipt.status, "preview-only");
  assert.equal(receipt.proof.gate, "mutation route blocked");
});
