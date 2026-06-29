import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import { spawnSync } from "node:child_process";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");
const skipDirs = new Set([".git", "node_modules", "coverage", ".local"]);
const forbiddenToolWord = "pi" + "lot";
const removedRuntimeWord = "py" + "thon";

async function walk(dir) {
  const out = [];
  for (const entry of await fs.readdir(dir, {withFileTypes: true})) {
    if (skipDirs.has(entry.name)) continue;
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) out.push(...await walk(full));
    else out.push(full);
  }
  return out;
}

const files = await walk(root);

for (const file of files) {
  assert(!file.endsWith(".py"), `removed runtime file remains: ${path.relative(root, file)}`);
  assert(!file.endsWith(".pyc"), `compiled removed runtime file remains: ${path.relative(root, file)}`);
  const text = await fs.readFile(file, "utf8").catch(() => "");
  assert(!text.toLowerCase().includes(forbiddenToolWord), `generator-tool wording remains in ${path.relative(root, file)}`);
  assert(!text.toLowerCase().includes(removedRuntimeWord), `removed runtime wording remains in ${path.relative(root, file)}`);
}

const testFiles = files
  .filter((file) => file.endsWith(".test.mjs"))
  .sort();

const result = spawnSync(process.execPath, ["--test", ...testFiles], {
  cwd: root,
  stdio: "inherit",
  env: {...process.env, DATA_MODE: "fixture"},
});

assert.equal(result.status, 0, "test suite failed");
console.log("addon-manager complete app verify: PASS");
