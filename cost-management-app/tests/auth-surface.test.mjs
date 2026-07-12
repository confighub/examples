import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const source = await readFile('public/auth.js', 'utf8');
const serverSource = await readFile('src/server.mjs', 'utf8');
const packageJson = await readFile('package.json', 'utf8');
assert.match(source, /ConfigHub Custom UI Apps JavaScript SDK/);
assert.doesNotMatch(source, /code_challenge|TokenExchangeEndpoint|openid-configuration/);
assert.match(source, /npm run ui:dev/);
assert.match(serverSource, /url\.pathname === '\/callback'/);
assert.match(serverSource, /callbackPath: '\/'/);
assert.match(packageJson, /oauth:register/);
assert.match(packageJson, /ui:dev/);
assert.match(packageJson, /@confighub\/react-auth/);
assert.match(packageJson, /@confighub\/api/);
