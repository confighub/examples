import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";

let bindings;
try {
  bindings = JSON.parse(await readFile("data/live-bindings.json", "utf8"));
} catch {
  console.error(JSON.stringify({
    status: "LIVE_BINDINGS_MISSING",
    requiredFile: "data/live-bindings.json",
    exampleFile: "data/live-bindings.example.json",
  }, null, 2));
  process.exit(1);
}

const required = [
  ["configHub", "objectUrl"],
  ["approval", "objectId"],
  ["action", "endpoint"],
  ["proof", "receiptObjectId"],
  ["runtime", "evidenceSource"],
];

for (const [section, field] of required) {
  assert.ok(bindings[section], `${section} section is required`);
  assert.ok(bindings[section][field], `${section}.${field} is required`);
}

console.log(JSON.stringify({
  status: "LIVE_BINDINGS_READY",
  configHubObject: bindings.configHub.objectUrl,
  actionEndpoint: bindings.action.endpoint,
  proofReceipt: bindings.proof.receiptObjectId,
}, null, 2));
