import { readFile } from 'node:fs/promises';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const workflowPath = join(here, '..', 'data', 'operational-workflow.json');

export async function readWorkflow() {
  const raw = await readFile(workflowPath, 'utf8');
  return JSON.parse(raw);
}
