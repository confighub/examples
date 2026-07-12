import { readFile } from 'node:fs/promises';
import { createAppServer } from './src/server.mjs';

let lifecycleState = null;
try {
  lifecycleState = JSON.parse(await readFile(new URL('./.lifecycle/state.json', import.meta.url), 'utf8'));
} catch {
  lifecycleState = null;
}
if (lifecycleState && lifecycleState.status === 'decommissioned') {
  console.error('DECOMMISSIONED: this app was decommissioned and deregistered from the fleet index.');
  console.error('Run node lifecycle.mjs rollback --json to restore it, or remove the app directory.');
  process.exit(3);
}

const server = createAppServer();
const port = Number(process.env.PORT || 5173);

server.listen(port, () => {
  console.log(`ConfigHub operational app listening on http://localhost:${port}`);
});
