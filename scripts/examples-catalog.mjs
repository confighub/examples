#!/usr/bin/env node
import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const catalogUrl = new URL('../catalog/examples.json', import.meta.url);
const catalog = JSON.parse(readFileSync(catalogUrl, 'utf8'));

export function eligible(entry) {
  return entry.visibility === 'public' &&
    entry.lifecycle === 'maintained' &&
    entry.role === 'walkthrough' &&
    entry.admission === 'verified-source';
}

function terms(value) {
  return value.toLowerCase().match(/[a-z0-9]+/g) ?? [];
}

function rank(entry, query) {
  const words = terms(query);
  const tags = entry.tags.join(' ').toLowerCase();
  const task = entry.task.toLowerCase();
  const id = entry.id.toLowerCase();
  if (words.length === 0) return 0;
  let score = 0;
  for (const word of words) {
    if (entry.tags.includes(word)) score += 8;
    else if (tags.includes(word)) score += 4;
    if (id.includes(word)) score += 5;
    if (task.includes(word)) score += 2;
  }
  return score;
}

export function findExamples({ query = '', includeAll = false, tag = '' } = {}) {
  const entries = catalog.entries.filter(entry =>
    (includeAll || eligible(entry)) && (!tag || entry.tags.includes(tag))
  );
  return entries
    .map(entry => ({ entry, score: query ? rank(entry, query) : 0 }))
    .filter(result => !query || result.score > 0)
    .sort((a, b) => b.score - a.score || a.entry.id.localeCompare(b.entry.id, 'en'))
    .map(result => result.entry);
}

export function getExample(id) {
  return catalog.entries.find(entry => entry.id === id);
}

function usage() {
  return `Usage:
  node scripts/examples-catalog.mjs list [--all] [--tag TAG] [--json]
  node scripts/examples-catalog.mjs search WORDS... [--all] [--tag TAG] [--json]
  node scripts/examples-catalog.mjs show ID [--json]

Output is JSON by default. list/search return {schema_version, entries}.
Default list/search includes only public maintained walkthroughs with verified
source paths. --all also includes clearly labelled candidates and references.
No command fetches a URL or runs an example.
`;
}

function main(args) {
  if (args.length === 0 || args.includes('--help') || args.includes('-h')) {
    process.stdout.write(usage());
    return;
  }
  const command = args.shift();
  let includeAll = false;
  let tag = '';
  const positionals = [];
  while (args.length) {
    const arg = args.shift();
    if (arg === '--all') includeAll = true;
    else if (arg === '--json') continue;
    else if (arg === '--tag' && args.length) tag = args.shift();
    else if (arg.startsWith('-')) throw new Error(`Unknown option: ${arg}`);
    else positionals.push(arg);
  }
  if (command === 'list' && positionals.length === 0) {
    process.stdout.write(JSON.stringify({ schema_version: catalog.schema_version, entries: findExamples({ includeAll, tag }) }, null, 2) + '\n');
  } else if (command === 'search' && positionals.length > 0) {
    process.stdout.write(JSON.stringify({ schema_version: catalog.schema_version, query: positionals.join(' '), entries: findExamples({ query: positionals.join(' '), includeAll, tag }) }, null, 2) + '\n');
  } else if (command === 'show' && positionals.length === 1 && !includeAll && !tag) {
    const entry = getExample(positionals[0]);
    if (!entry) throw new Error(`Unknown example ID: ${positionals[0]}`);
    process.stdout.write(JSON.stringify({ schema_version: catalog.schema_version, entry }, null, 2) + '\n');
  } else {
    throw new Error('Invalid command or arguments. Use --help for usage.');
  }
}

if (process.argv[1] && fileURLToPath(import.meta.url) === resolve(process.argv[1])) {
  try {
    main(process.argv.slice(2));
  } catch (error) {
    process.stderr.write(`${error.message}\n`);
    process.exitCode = 1;
  }
}
