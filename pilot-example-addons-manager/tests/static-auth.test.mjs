import assert from "node:assert/strict";
import fs from "node:fs/promises";
import path from "node:path";
import test from "node:test";
import { fileURLToPath } from "node:url";

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), "..");

test("browser auth helper implements discovery, PKCE, token exchange, and bearer API calls", async () => {
  const auth = await fs.readFile(path.join(root, "public", "auth.js"), "utf8");
  assert.match(auth, /\/api\/info/);
  assert.match(auth, /TokenExchangeEndpoint/);
  assert.match(auth, /code_challenge_method/);
  assert.match(auth, /S256/);
  assert.match(auth, /urn:ietf:params:oauth:grant-type:token-exchange/);
  assert.match(auth, /authorization: `Bearer/);
});

test("browser ConfigHub client calls inventory endpoints directly", async () => {
  const client = await fs.readFile(path.join(root, "public", "confighub-api.js"), "utf8");
  assert.match(client, /\/api\/space/);
  assert.match(client, /\/unit/);
  assert.match(client, /\/data/);
  assert.match(client, /\/revision/);
  assert.match(client, /browser-oauth/);
});
