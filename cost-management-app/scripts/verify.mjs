import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';

const forbidden = ['pi' + 'lot', 'py' + 'thon'];
const skipDirs = new Set(['.git', 'node_modules', '.lifecycle']);
// Deployment-local live data (gitignored) carries the customer org's own
// naming in space and unit slugs; the vocabulary fence covers shipped app
// content, not the customer's data about itself.
const skipFiles = new Set(['data/live-bindings.json', 'data/cost-findings.json', 'package-lock.json']);
const skipPrefixes = ['data/approvals/', 'data/receipts/'];

async function files(dir) {
  const entries = await readdir(dir, {withFileTypes: true});
  const result = [];
  for (const entry of entries) {
    if (skipDirs.has(entry.name)) continue;
    const full = join(dir, entry.name);
    if (entry.isDirectory()) {
      result.push(...await files(full));
    } else {
      result.push(full);
    }
  }
  return result;
}

for (const file of await files('.')) {
  if (skipFiles.has(file) || skipPrefixes.some(prefix => file.startsWith(prefix))) continue;
  assert.ok(!file.endsWith('.py'), `unexpected server runtime file: ${file}`);
  const text = await readFile(file, 'utf8');
  const lower = text.toLowerCase();
  for (const word of forbidden) {
    assert.ok(!lower.includes(word), `forbidden cue ${word} in ${file}`);
  }
}
