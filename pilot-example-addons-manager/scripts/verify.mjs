import assert from 'node:assert/strict';
import { readdir, readFile } from 'node:fs/promises';
import { join } from 'node:path';

const forbidden = ['pi' + 'lot', 'py' + 'thon'];
const skipDirs = new Set(['.git', 'node_modules']);

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
  assert.ok(!file.endsWith('.py'), `unexpected server runtime file: ${file}`);
  const text = await readFile(file, 'utf8');
  const lower = text.toLowerCase();
  for (const word of forbidden) {
    assert.ok(!lower.includes(word), `forbidden cue ${word} in ${file}`);
  }
}
