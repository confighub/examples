import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';

const packageJson = JSON.parse(await readFile('package.json', 'utf8'));
const workflow = JSON.parse(await readFile('data/operational-workflow.json', 'utf8'));
const manifest = JSON.parse(await readFile('app-export-manifest.json', 'utf8'));
const index = await readFile('index.html', 'utf8');
const vite = await readFile('vite.config.ts', 'utf8');
const tsconfig = await readFile('tsconfig.json', 'utf8');
const main = await readFile('ui/src/main.tsx', 'utf8');
const app = await readFile('ui/src/App.tsx', 'utf8');

assert.equal(workflow.uiTool.name, 'ConfigHub Custom UI Apps JavaScript SDK');
assert.equal(workflow.uiTool.reactProvider, 'ConfigHubAuthProvider');
assert.equal(workflow.uiTool.authHook, 'useAuth');
assert.equal(workflow.uiTool.apiHook, 'useConfigHub');
assert.equal(workflow.uiTool.redirectUri, 'http://localhost:5173/');
assert.equal(manifest.ui_tool.name, workflow.uiTool.name);
assert.equal(packageJson.dependencies['@confighub/react-auth'], '^0.1.0');
assert.equal(packageJson.dependencies['@confighub/api'], '^0.1.0');
assert.equal(packageJson.scripts['ui:dev'], 'vite --host 127.0.0.1 --port 5173 --config vite.config.ts');
assert.match(index, /ui\/src\/main\.tsx/);
assert.match(vite, /@vitejs\/plugin-react/);
assert.match(tsconfig, /resolveJsonModule/);
assert.match(main, /ConfigHubAuthProvider/);
assert.match(main, /VITE_CONFIGHUB_BASE_URL/);
assert.match(main, /VITE_OAUTH_CLIENT_ID/);

// First screen: before any sign-in the app must lead with its purpose (from
// the workflow contract), say plainly that nothing is live, and offer one
// human next action. Developer mechanics stay demoted to a collapsed detail.
assert.match(main, /operational-workflow\.json/);
assert.match(main, /workflow\.app\.name/);
assert.match(main, /workflow\.scenario\.jobToBeDone/);
assert.match(main, /Not yet connected to ConfigHub\. Nothing here is live\./);
assert.match(main, /has not been registered yet/);
assert.match(main, /<details/);
assert.match(main, /npm run oauth:register/);
assert.doesNotMatch(main, /OAuth client required/);
assert.match(app, /Not yet connected to ConfigHub\. Nothing here is live\./);
assert.match(app, /Sign in<\/button>/);

assert.match(app, /useAuth/);
assert.match(app, /useConfigHub/);
assert.match(app, /api\.GET\('\/me'\)/);
assert.match(app, /operational-workflow\.json/);
assert.match(app, /workflow\.variants/);

// Session lifecycle: the auth provider owns silent re-auth and every piece of
// browser-storage state behind it. App code must not fight the provider by
// persisting tokens, touching browser storage, or hand-rolling any part of
// the OAuth flow.
for (const uiSource of [main, app]) {
  assert.doesNotMatch(uiSource, /localStorage|sessionStorage|indexedDB/);
  assert.doesNotMatch(uiSource, /code_challenge|TokenExchangeEndpoint|openid-configuration/);
}
