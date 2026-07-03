import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { extname, join, normalize } from 'node:path';
import { dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { readRuntimeConfig } from './config.mjs';
import { readWorkflow } from './workflow.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const root = join(here, '..');
const publicDir = join(root, 'public');

const contentTypes = {
  '.html': 'text/html; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.json': 'application/json; charset=utf-8',
  '.svg': 'image/svg+xml; charset=utf-8',
};

function sendJson(res, status, value) {
  res.writeHead(status, {'content-type': 'application/json; charset=utf-8'});
  res.end(JSON.stringify(value, null, 2));
}

function readBody(req) {
  return new Promise((resolve, reject) => {
    let data = '';
    req.on('data', chunk => { data += chunk; });
    req.on('end', () => {
      if (!data) {
        resolve({});
        return;
      }
      try {
        resolve(JSON.parse(data));
      } catch (error) {
        reject(error);
      }
    });
  });
}

function hasBearer(req) {
  const auth = req.headers.authorization || '';
  return auth.startsWith('Bearer ');
}

async function readLiveBindings() {
  try {
    const raw = await readFile(join(root, 'data', 'live-bindings.json'), 'utf8');
    const bindings = JSON.parse(raw);
    if (hasPlaceholder(bindings)) {
      return {
        status: 'LIVE_BINDINGS_PLACEHOLDER',
        bindings,
        requiredFile: 'data/live-bindings.json',
        exampleFile: 'data/live-bindings.example.json',
        reason: 'The live binding file still contains example placeholder values.',
      };
    }
    const contractError = bindingContractError(bindings);
    if (contractError) {
      return {
        status: 'LIVE_BINDINGS_INCOMPLETE',
        bindings,
        requiredFile: 'data/live-bindings.json',
        exampleFile: 'data/live-bindings.example.json',
        reason: contractError,
      };
    }
    return {status: 'LIVE_BINDINGS_PRESENT', bindings};
  } catch {
    return {
      status: 'LIVE_BINDINGS_MISSING',
      bindings: null,
      requiredFile: 'data/live-bindings.json',
      exampleFile: 'data/live-bindings.example.json',
    };
  }
}

function hasPlaceholder(value) {
  if (typeof value === 'string') {
    return value.includes('<') || value.includes('>') || value.includes('example-fill');
  }
  if (Array.isArray(value)) {
    return value.some(hasPlaceholder);
  }
  if (value && typeof value === 'object') {
    return Object.values(value).some(hasPlaceholder);
  }
  return false;
}

function bindingContractError(bindings) {
  const contract = bindings?.action?.contract;
  if (!contract) return 'action.contract is required';
  if (contract.kind !== 'ConfigHub-governed-action.v0') return 'action.contract.kind must be ConfigHub-governed-action.v0';
  if (!contract.operation) return 'action.contract.operation is required';
  if (!Array.isArray(contract.scopeFields)) return 'action.contract.scopeFields must be an array';
  if (!Array.isArray(contract.requires)) return 'action.contract.requires must be an array';
  return '';
}

async function serveStatic(req, res, url) {
  const requested = url.pathname === '/' || url.pathname === '/callback'
    ? '/index.html'
    : url.pathname;
  const safePath = normalize(requested).replace(/^\.{2,}/, '');
  const filePath = join(publicDir, safePath);
  if (!filePath.startsWith(publicDir)) {
    sendJson(res, 403, {error: 'FORBIDDEN'});
    return;
  }
  try {
    const body = await readFile(filePath);
    res.writeHead(200, {'content-type': contentTypes[extname(filePath)] || 'application/octet-stream'});
    res.end(body);
  } catch {
    sendJson(res, 404, {error: 'NOT_FOUND'});
  }
}

function authBlocked(req, workflow) {
  return workflow.authMode === 'browser-oauth' && !hasBearer(req);
}

export function createAppServer() {
  return createServer(async (req, res) => {
    const url = new URL(req.url || '/', 'http://localhost');
    const workflow = await readWorkflow();
    const runtime = readRuntimeConfig();
    workflow.authMode = runtime.authMode;

    try {
      if (req.method === 'GET' && url.pathname === '/app/config') {
        sendJson(res, 200, {
          authMode: runtime.authMode,
          configHubBaseUrl: runtime.configHubBaseUrl,
          oauthClientId: runtime.oauthClientId,
          callbackPath: '/callback',
          workflowStatus: workflow.status,
        });
        return;
      }

      if (req.method === 'GET' && url.pathname === '/api/workflow') {
        sendJson(res, 200, workflow);
        return;
      }

      if (req.method === 'GET' && url.pathname === '/api/variants') {
        sendJson(res, 200, {variants: workflow.variants});
        return;
      }

      if (req.method === 'GET' && url.pathname.startsWith('/api/variants/')) {
        const id = decodeURIComponent(url.pathname.split('/').pop() || '');
        const variant = workflow.variants.find(item => item.id === id);
        sendJson(res, variant ? 200 : 404, variant || {error: 'VARIANT_NOT_FOUND'});
        return;
      }

      if (req.method === 'GET' && url.pathname === '/api/receipt') {
        const liveBindings = await readLiveBindings();
        sendJson(res, 200, {
          status: 'WAITING_FOR_LIVE_PROOF',
          app: workflow.app.name,
          checks: workflow.proofTabs.map(tab => ({id: tab.id, label: tab.label, status: tab.status})),
          liveBindings,
          stopRules: workflow.stopRules,
        });
        return;
      }

      if (req.method === 'GET' && url.pathname === '/api/bindings') {
        sendJson(res, 200, await readLiveBindings());
        return;
      }

      if (req.method === 'POST' && url.pathname === '/api/preview') {
        if (authBlocked(req, workflow)) {
          sendJson(res, 401, {error: 'SIGN_IN_REQUIRED'});
          return;
        }
        const body = await readBody(req);
        sendJson(res, 200, {
          status: 'PREVIEW_READY',
          variantId: body.variantId || workflow.variants[0]?.id,
          message: 'Scope preview ready. Review approval before any live operation.',
        });
        return;
      }

      if (req.method === 'POST' && url.pathname === '/api/approval') {
        if (authBlocked(req, workflow)) {
          sendJson(res, 401, {error: 'SIGN_IN_REQUIRED'});
          return;
        }
        const body = await readBody(req);
        sendJson(res, 200, {
          status: 'APPROVAL_RECORDED_LOCALLY',
          variantId: body.variantId || workflow.variants[0]?.id,
          scopeFields: workflow.approval.scopeFields,
          nextGate: 'Bind this approval to the live ConfigHub action before apply.',
        });
        return;
      }

      if (req.method === 'POST' && url.pathname === '/api/apply') {
        if (authBlocked(req, workflow)) {
          sendJson(res, 401, {error: 'SIGN_IN_REQUIRED'});
          return;
        }
        const liveBindings = await readLiveBindings();
        if (liveBindings.status !== 'LIVE_BINDINGS_PRESENT') {
          sendJson(res, 409, {
            error: 'LIVE_BINDINGS_REQUIRED',
            message: 'Create data/live-bindings.json before running a live operation.',
            liveBindings,
          });
          return;
        }
        sendJson(res, 409, {
          error: 'LIVE_ACTION_EXECUTOR_REQUIRED',
          message: 'Bindings are present, but the scenario-specific ConfigHub action executor is not implemented in this export.',
        });
        return;
      }

      if (req.method === 'GET') {
        await serveStatic(req, res, url);
        return;
      }

      sendJson(res, 405, {error: 'METHOD_NOT_ALLOWED'});
    } catch (error) {
      sendJson(res, 500, {error: 'SERVER_ERROR', message: String(error.message || error)});
    }
  });
}
