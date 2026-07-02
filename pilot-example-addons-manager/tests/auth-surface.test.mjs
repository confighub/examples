import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const source = await readFile('public/auth.js', 'utf8');
assert.match(source, /AuthIssuer/);
assert.match(source, /openid-configuration/);
assert.match(source, /TokenExchangeEndpoint/);
assert.match(source, /tokenEndpoint/);
assert.match(source, /code_challenge_method/);
assert.match(source, /S256/);
assert.match(source, /Authorization|authorization/);
assert.match(source, /urn:ietf:params:oauth:grant-type:token-exchange/);
assert.match(source, /api\/info/);
