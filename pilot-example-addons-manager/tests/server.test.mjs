import assert from "node:assert/strict";
import test from "node:test";
import { createAppServer, listen } from "../src/server.mjs";

async function withServer(fn) {
  const server = createAppServer({dataMode: "fixture", port: 0});
  const address = await listen(server, 0);
  const base = `http://127.0.0.1:${address.port}`;
  try {
    await fn(base);
  } finally {
    await new Promise((resolve) => server.close(resolve));
  }
}

async function getJson(base, path) {
  const response = await fetch(base + path);
  assert.equal(response.ok, true, `${path} returned ${response.status}`);
  return response.json();
}

test("serves the browser app shell", async () => {
  await withServer(async (base) => {
    const response = await fetch(base + "/");
    const html = await response.text();
    assert.equal(response.status, 200);
    assert.match(html, /Add-on Manager/);
    assert.match(html, /Add-ons by Variant/);
    assert.match(html, /Approval And Proof/);
  });
});

test("serves workflow and fixture inventory", async () => {
  await withServer(async (base) => {
    const workflow = await getJson(base, "/api/workflow?mode=fixture");
    assert.equal(workflow.app, "Add-on Manager");
    assert.equal(workflow.steps.length, 5);
    assert(workflow.proofTabs.includes("Receipt"));

    const inventory = await getJson(base, "/api/inventory?mode=fixture");
    assert.equal(inventory.source, "fixture");
    assert.equal(inventory.totals.addons, 3);
    assert.equal(inventory.totals.units, 8);
  });
});

test("serves detail, proposal, and receipt endpoints", async () => {
  await withServer(async (base) => {
    const query = "space=helm-kyverno-prod&unit=kyverno-controller&mode=fixture";
    const detail = await getJson(base, `/api/detail?${query}`);
    assert.equal(detail.unit.slug, "kyverno-controller");
    assert.equal(detail.warnings.length, 1);

    const proposal = await getJson(base, `/api/proposal?${query}`);
    assert.equal(proposal.approvalScope.variant, "prod");
    assert.equal(proposal.previewOnly, true);

    const receipt = await getJson(base, `/api/receipt?${query}`);
    assert.equal(receipt.status, "preview-only");
    assert.equal(receipt.proof.controller, "not connected");
  });
});

test("write routes are blocked", async () => {
  await withServer(async (base) => {
    const response = await fetch(base + "/api/apply", {method: "POST"});
    const payload = await response.json();
    assert.equal(response.status, 405);
    assert.equal(payload.blocked, true);
    assert.match(payload.error, /read-only sample app/);
  });
});
