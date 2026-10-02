#!/usr/bin/env node
// No personal names in committed files.
//
// Prose in this repository says "the team", "a colleague" or the issue number
// instead of naming a person; a home directory path is written $HOME and a
// sample identity user@example.com. The tree is clean today. This check keeps
// it clean: it fails when a tracked file contains one of the first names below
// as a whole word, in any letter case, and prints the lines.
//
//   - git grep -w matches a whole word, so a longer identifier that merely
//     contains a name (a GitHub handle, say) is not a hit.
//   - This file holds the pattern, so it is the one file left out of the scan.
//     Keep names out of its prose too.
//
// Usage:
//   node scripts/verify-no-personal-names.mjs   # exit 1 on any hit
import { spawnSync } from "node:child_process";
import { dirname } from "node:path";
import { fileURLToPath } from "node:url";

const repoRoot = dirname(dirname(fileURLToPath(import.meta.url)));
const PATTERN = "alexis|jesper|brian|charlie";
const SELF = "scripts/verify-no-personal-names.mjs";

const res = spawnSync(
  "git",
  ["grep", "-nwiE", PATTERN, "--", ".", `:(exclude)${SELF}`],
  { cwd: repoRoot, encoding: "utf8" },
);

if (res.error) throw res.error;

// git grep exits 1 when nothing matches.
if (res.status === 1) {
  console.log("no-personal-names: clean (every tracked file)");
  process.exit(0);
}

if (res.status === 0) {
  const hits = res.stdout.trim();
  console.error(hits);
  console.error("");
  console.error(`no-personal-names: ${hits.split("\n").length} line(s) above name a person in a committed file.`);
  console.error("Replace each name with neutral wording (the team / a colleague / the issue number).");
  process.exit(1);
}

console.error(res.stderr.trim() || `git grep exited with status ${res.status}`);
process.exit(2);
