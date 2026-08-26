// A production build that succeeds is not a working app. webkit resolves its bare
// imports from webkit/node_modules while the console resolves the same names from its
// own, so a package can end up in the bundle twice — and a package with module state,
// like @confighub/react-auth's React context, silently stops working when it does.
// Nothing in `tsc` or `vite build` reports this; only the source map does.
//
// Usage: node scripts/check-bundle-dedupe.mjs <app-dir>
//   (the app must have been built with --sourcemap)

import { readFileSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

const appDir = process.argv[2];
if (!appDir) {
  console.error('usage: node scripts/check-bundle-dedupe.mjs <app-dir>');
  process.exit(2);
}

const assets = join(appDir, 'dist', 'assets');
let maps;
try {
  maps = readdirSync(assets).filter((n) => n.endsWith('.js.map'));
} catch {
  console.error(`${appDir}: no dist/assets — build with --sourcemap first`);
  process.exit(2);
}
if (maps.length === 0) {
  console.error(`${appDir}: no source maps — build with --sourcemap first`);
  process.exit(2);
}

// Which node_modules *root* each package came from — the path before the first
// node_modules, so webkit's tree and the console's are distinguished. Nesting inside
// one root (npm resolving two versions of a transitive dep) is normal and ignored;
// the same package under two roots is the failure this catches.
const roots = new Map();
for (const map of maps) {
  const { sources = [] } = JSON.parse(readFileSync(join(assets, map), 'utf8'));
  for (const source of sources) {
    const first = source.indexOf('node_modules/');
    if (first === -1) continue;
    const root = source.slice(0, first);
    const last = source.lastIndexOf('node_modules/') + 'node_modules/'.length;
    const rest = source.slice(last);
    const pkg = rest.startsWith('@') ? rest.split('/').slice(0, 2).join('/') : rest.split('/')[0];
    if (!pkg) continue;
    if (!roots.has(pkg)) roots.set(pkg, new Set());
    roots.get(pkg).add(root);
  }
}

const duplicated = [...roots.entries()]
  .filter(([, prefixes]) => prefixes.size > 1)
  .map(([pkg, prefixes]) => `  ${pkg}\n${[...prefixes].map((p) => `    from ${p || './'}node_modules/`).join('\n')}`);

if (duplicated.length > 0) {
  console.error(`${appDir}: these packages are in the bundle more than once:\n${duplicated.join('\n')}`);
  console.error(`\nAdd them to resolve.dedupe in ${appDir}/vite.config.ts.`);
  process.exit(1);
}

console.log(`${appDir}: no duplicated packages in the bundle`);
